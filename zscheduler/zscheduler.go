package zprocess

import (
	"context"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zslice"
)

// A *Scheduler* starts *Job*s on *Executor*s, trying to balance the workload.
// Each Job has a *Cost*, and each executor a *CostCapacity*.
// Jobs can have a Duration or go until stopped.
// The scheduler assumes jobs take a considerable time to start and end,
// and can get congested if too many are starting at once, so has
// SimultaneousStarts and MinDurationBetweenSimultaneousStarts parameters.
// With these constraints, priority is to start jobs as soon as possible on any executor with enough capacity.
// Once running, a job may be load-balanced to a different executor if capacity difference < LoadBalanceIfCostDifference.
// All changes to a Scheduler are done through channels
type Scheduler[I comparable] struct {
	ExecutorAliveDuration                time.Duration
	SimultaneousStarts                   int
	LoadBalanceIfCostDifference          float64
	MinDurationBetweenSimultaneousStarts time.Duration
	KeepJobsBeyondAtEndUntilEnoughSlack  time.Duration
	SlowFuncsTimeout                     time.Duration

	StartJobOnExecutorSlowFunc func(run Run[I], ctx context.Context) error // this function can be slow, like a request. If it errors it can be run again n times until RepeatFuncsWithBackoffUpTo waiting between
	StopJobOnExecutorSlowFunc  func(run Run[I], ctx context.Context) error
	HandleSituationFastFunc    func(run Run[I], s SituationType, err error) // this function must very quickly do something or spawn a go routine

	// The channels are made in NewScheduler()
	StopJobCh            chan I
	RemoveJobCh          chan I
	AddJobCh             chan Job[I]
	ChangeJobCh          chan Job[I]
	JobIsRunningCh       chan I
	AddExecutorCh        chan Executor[I]
	RemoveExecutorCh     chan I
	ChangeExecutorCh     chan Executor[I]
	SetExecutorIsAliveCh chan I
	SetJobsOnExecutorCh  chan JobsOnExecutor[I]

	refreshCh   chan struct{}
	endRunCh    chan I // clearJobCh sets a run to after stopped status
	removeRunCh chan I // removeRunCh does local remove of a run

	executors []Executor[I]
	runs      []Run[I]
	runCount  int
	zeroID    I
	timer     *time.Timer
	Debug     zmap.LockMap[I, JobDebug]
}

type Job[I comparable] struct {
	ID        I
	DebugName string
	Duration  time.Duration // How long job should run for. 0 is until stopped.
	Cost      float64       // Cost is how much of executor's CostCapacity job uses.
}

type Executor[I comparable] struct {
	ID           I
	Paused       bool
	CostCapacity float64
	KeptAliveAt  time.Time
	DebugName    string
}

type Run[I comparable] struct {
	Job        Job[I]
	ExecutorID I
	Count      int
	// Accepted   bool
	StartedAt time.Time
	RanAt     time.Time
	StoppedAt time.Time
	// EndedAt    time.Time
	Removing bool
	Stopping bool
}

type JobsOnExecutor[I comparable] struct {
	JobIDs     []I
	ExecutorID I
}

type SituationType string

const (
	NoWorkersToRunJob           SituationType = "no workers fit to run job"
	RemoveJobFromExecutorFailed SituationType = "remove job from exector failed"
	ErrorStartingJob            SituationType = "error starting job"

	JobStarted SituationType = "job started"
	JobRunning SituationType = "job running"
	JobStopped SituationType = "job stopped"
	JobEnded   SituationType = "job ended"
)

func NewScheduler[I comparable]() *Scheduler[I] {
	zlog.Warn("NewScheduler")
	b := &Scheduler[I]{}

	b.AddJobCh = make(chan Job[I])
	b.RemoveJobCh = make(chan I)
	b.StopJobCh = make(chan I)
	b.ChangeJobCh = make(chan Job[I])
	b.JobIsRunningCh = make(chan I)
	b.AddJobCh = make(chan Job[I])
	b.SetExecutorIsAliveCh = make(chan I)
	b.ChangeExecutorCh = make(chan Executor[I])
	b.AddExecutorCh = make(chan Executor[I])
	b.RemoveExecutorCh = make(chan I)
	b.AddExecutorCh = make(chan Executor[I])
	b.refreshCh = make(chan struct{})
	b.endRunCh = make(chan I)

	b.ExecutorAliveDuration = time.Second * 10
	b.SimultaneousStarts = 1
	b.MinDurationBetweenSimultaneousStarts = 0
	b.KeepJobsBeyondAtEndUntilEnoughSlack = 0

	return b
}

func (b *Scheduler[I]) findRun(jobID I) (*Run[I], int) {
	for i, r := range b.runs {
		if r.Job.ID == jobID {
			return &b.runs[i], i
		}
	}
	return nil, -1
}

func (b *Scheduler[I]) findExecutor(id I) (*Executor[I], int) {
	for i, e := range b.executors {
		if e.ID == id {
			return &b.executors[i], i
		}
	}
	return nil, -1
}

func (b *Scheduler[I]) stopJob(jobID I, remove, outsideRequest bool) {
	now := time.Now()
	run, _ := b.findRun(jobID)
	if run == nil {
		zlog.Error(nil, "Scheduler stop: no run with that id", jobID)
		zlog.Assert(outsideRequest)
		return
	}
	if run.Stopping {
		zlog.Assert(outsideRequest)
		zlog.Warn("Bailing stopJob on stopping already", zlog.Full(run))
		if remove && !run.Removing {
			zlog.Warn("Upgrading stop to remove from outsite stop+remove", run.Job.DebugName)
		}
		return
	}
	// zlog.Warn("stopJob", run.Started.IsZero(), "remove:", remove, run.Job.DebugName, run.ExecutorID) //, zlog.CallingStackString())
	b.setDebugState(jobID, false, false, true, false)
	run.Stopping = true
	run.StoppedAt = now
	run.Removing = remove
	run.RanAt = time.Time{}
	run.StartedAt = time.Time{}
	r := *run
	// zlog.Warn("stopJob handleSit:", r.Job.ID, r.ExecutorID)
	b.HandleSituationFastFunc(r, JobStopped, nil)
	// run.ExecutorID = b.zeroID
	ctx, _ := context.WithDeadline(context.Background(), now.Add(b.SlowFuncsTimeout))
	go func() {
		err := b.StopJobOnExecutorSlowFunc(r, ctx)
		b.setDebugState(jobID, !remove, false, false, false)
		if err != nil {
			b.HandleSituationFastFunc(r, RemoveJobFromExecutorFailed, err)
		}
		b.endRunCh <- jobID
	}()
	b.startAndStopRuns()
}

func (b *Scheduler[I]) addJob(job Job[I], outsideRequest bool) {
	_, i := b.findRun(job.ID)
	if i != -1 {
		// zlog.Error(nil, "adding job when existing", job.DebugName, i)
		zlog.Assert(outsideRequest)
		return
	}
	b.setDebugState(job.ID, true, false, false, false)
	d, _ := b.Debug.Get(job.ID)
	d.JobName = job.DebugName
	b.Debug.Set(job.ID, d)

	var run Run[I]
	run.Job = job
	run.Count = b.runCount
	b.runCount++
	b.runs = append(b.runs, run)
	b.startAndStopRuns()
}

func (b *Scheduler[I]) runnableExecutorIDs() map[I]bool {
	m := map[I]bool{}
	for _, e := range b.executors {
		// zlog.Warn("runnableExecutorIDs:", e.Paused, e.KeptAliveAt, b.isExecutorAlive(&e))
		if !e.Paused && b.isExecutorAlive(&e) {
			m[e.ID] = true
		}
	}
	return m
}

func (b *Scheduler[I]) hasUnrunJobs() bool {
	for _, r := range b.runs {
		if r.ExecutorID == b.zeroID || r.Stopping || r.Removing || r.StartedAt.IsZero() {
			return true
		}
	}
	return false
}

func (b *Scheduler[I]) maybeStopJobOnPaused(run Run[I]) bool {
	if b.hasUnrunJobs() {
		if b.KeepJobsBeyondAtEndUntilEnoughSlack == 0 {
			return false
		}

	}
	b.stopJob(run.Job.ID, false, false)
	return true
}

func (b *Scheduler[I]) isJobOverdueToQuit(run Run[I]) (quit, over bool) {
	if run.Job.Duration == 0 {
		return false, false
	}
	if run.RanAt.IsZero() {
		return false, false
	}
	overEnd := time.Since(run.RanAt) - run.Job.Duration
	if overEnd < 0 {
		return false, false
	}
	if b.KeepJobsBeyondAtEndUntilEnoughSlack == 0 {
		// zlog.Warn("isJobOverdueToQuit here", run.Job.ID, run.RanAt, overEnd)
		return true, true
	}
	if overEnd < b.KeepJobsBeyondAtEndUntilEnoughSlack {
		return false, true
	}
	// zlog.Warn("isJobOverdueToQuit here2", run.Job.ID)
	return true, true
}

func (b *Scheduler[I]) startAndStopRuns() bool {
	var oldestRun *Run[I]
	var paused bool
	// zlog.Warn("startAndStopRuns")
	for i, r := range b.runs {
		// zlog.Warn("startAndStopRuns", r.Job.ID, r.ExecutorID)
		if r.ExecutorID != b.zeroID {
			var quit, over bool
			e, _ := b.findExecutor(r.ExecutorID)
			if e != nil {
				quit, over = b.isJobOverdueToQuit(r)
			}
			if e == nil || quit || !b.isExecutorAlive(e) {
				// zlog.Warn("startAndStopRuns stop:", e == nil, r.Job.ID, quit, b.isExecutorAlive(e))
				b.stopJob(r.Job.ID, false, false)
				continue
			}
			if e.Paused || over {
				if !paused {
					paused = b.maybeStopJobOnPaused(r)
				}
			}
			continue
		}
		// zlog.Warn("startAndStopRuns", r.Removing, r.StartedAt, r.Job.ID, r.ExecutorID)
		if r.ExecutorID == b.zeroID && !r.Stopping && r.StartedAt.IsZero() {
			if oldestRun == nil || r.StoppedAt.Sub(oldestRun.StoppedAt) > 0 {
				oldestRun = &b.runs[i]
			}
		}
	}
	// zlog.Warn("startAndStop?:", oldestRun != nil)
	if oldestRun != nil {
		b.startJob(oldestRun)
	}
	var nextTimerTime time.Time
	var timerJob string
	for _, r := range b.runs {
		if r.Job.Duration == 0 {
			continue
		}
		if !r.RanAt.IsZero() {
			// zlog.Warn("set timer has run")
			jobEnd := r.RanAt.Add(r.Job.Duration)
			if nextTimerTime.IsZero() || nextTimerTime.Sub(jobEnd) > 0 {
				timerJob = r.Job.DebugName
				nextTimerTime = jobEnd
			}
		}
	}
	if !nextTimerTime.IsZero() {
		d := -time.Since(nextTimerTime)
		// zlog.Warn("SetNextTimer:", nextTimerTime)
		zlog.Assert(d > -time.Second, d, timerJob)
		b.timer.Stop()
		b.timer.Reset(d)
	}
	return false
}

type capacity struct {
	load               float64
	startingCount      int
	mostRecentStarting time.Time //!!!!!!!!!!!!!! use this to not run 2 jobs on same worker after each other!
}

func (b *Scheduler[I]) calculateLoadOfUsableExecutors() map[I]capacity {
	m := map[I]capacity{}
	runnableEx := b.runnableExecutorIDs()
	for _, e := range b.executors {
		if !runnableEx[e.ID] {
			continue
		}
		m[e.ID] = capacity{}
	}
	for _, r := range b.runs {
		if !runnableEx[r.ExecutorID] {
			continue
		}
		c := m[r.ExecutorID]
		if !r.Stopping {
			if !r.StartedAt.IsZero() {
				c.load += r.Job.Cost
				if r.RanAt.IsZero() {
					c.startingCount++
				}
				if r.StartedAt.Sub(c.mostRecentStarting) > 0 {
					c.mostRecentStarting = r.StartedAt
				}
			}
			m[r.ExecutorID] = c
			// zlog.Warn("here", r.ExecutorID, zlog.Full(c))
		}
	}
	// for e, c := range m {
	// 	zlog.Warn("calculateLoadOfUsableExecutors calc startingCount:", e, c.startingCount, c.load)
	// }
	return m
}

func (b *Scheduler[I]) startJob(run *Run[I]) {
	jobID := run.Job.ID
	var bestExID I
	var bestStartingCount = -1
	// var bestCapacity float64
	var bestFull float64
	now := time.Now()
	m := b.calculateLoadOfUsableExecutors()
	// zlog.Warn("startJob1?:", run.Job.ID, len(m))
	var str string
	for exID, cap := range m {
		e, _ := b.findExecutor(exID)
		exCap := e.CostCapacity - cap.load
		exFull := cap.load / e.CostCapacity
		if exCap < run.Job.Cost {
			continue
		}
		// zlog.Warn("startJob?:", run.Job.DebugName, exID, zlog.Full(cap), b.SimultaneousStarts, b.MinDurationBetweenSimultaneousStarts)
		if cap.startingCount >= b.SimultaneousStarts {
			continue
		}
		if cap.startingCount > 0 && b.SimultaneousStarts > 1 {
			if time.Since(cap.mostRecentStarting) < b.MinDurationBetweenSimultaneousStarts {
				continue
			}
		}
		str += fmt.Sprint(" • ex:", exID, exFull, cap.load, e.CostCapacity)
		if bestStartingCount == -1 || cap.startingCount <= bestStartingCount || exFull < bestFull { // exCap > bestCapacity {
			if bestStartingCount == -1 {
				str += " FirstCapacity "
			}
			str += fmt.Sprint(" AnyStartingCount:", cap.startingCount)
			if cap.startingCount < bestStartingCount {
				str += fmt.Sprint(" BestStartingCount:", cap.startingCount, "<", bestStartingCount)
			}
			if exFull < bestFull {
				str += fmt.Sprint(" BestFull:", exFull)
			}
			// if exCap > bestCapacity {
			// 	str += fmt.Sprint(" BestCapacity:", exCap)
			// }
			// above: We prioritize number of current starting over capacity
			// bestCapacity = exCap
			bestFull = exFull
			bestExID = exID
			bestStartingCount = cap.startingCount
		}
	}
	if bestExID == b.zeroID {
		b.HandleSituationFastFunc(*run, NoWorkersToRunJob, zlog.NewError("job:", run.Job.ID))
		return
	}
	e, _ := b.findExecutor(bestExID)
	// zlog.Warn("startJob:", run.Job.DebugName, e.DebugName, str)
	b.setDebugState(run.Job.ID, false, true, false, false)
	run.StoppedAt = time.Time{}
	run.Removing = false
	run.StartedAt = now
	// run.Starting = true
	run.ExecutorID = bestExID
	d, _ := b.Debug.Get(run.Job.ID)
	d.ExecutorName = e.DebugName
	b.Debug.Set(run.Job.ID, d)
	b.HandleSituationFastFunc(*run, JobStarted, nil)
	ctx, _ := context.WithDeadline(context.Background(), now.Add(b.SlowFuncsTimeout))
	go func() {
		err := b.StartJobOnExecutorSlowFunc(*run, ctx)
		r, _ := b.findRun(jobID)
		zlog.Assert(r != nil, jobID)
		if err != nil {
			zlog.Warn(r.Job.DebugName, "started end err", err)
			b.endRunCh <- r.Job.ID
			b.HandleSituationFastFunc(*run, ErrorStartingJob, err)
		} else {
			b.JobIsRunningCh <- jobID
		}
		b.refreshCh <- struct{}{}
	}()
	go func() {
		b.refreshCh <- struct{}{} // let's do this via a channel, or we can get recursive, which looks weird when debugging
	}()
}

func (b *Scheduler[I]) addExector(e Executor[I]) {
	zlog.Warn("addExecutor")
	b.executors = append(b.executors, e)
	b.startAndStopRuns()
}

func (b *Scheduler[I]) Start() {
	b.timer = time.NewTimer(0)
	for {
		select {
		case j := <-b.AddJobCh:
			b.addJob(j, true)

		case jobID := <-b.RemoveJobCh:
			// zlog.Warn("RemoveJobCh", jobID)
			b.stopJob(jobID, true, true)

		case jobID := <-b.StopJobCh:
			// zlog.Warn("StopJobCh", jobID)
			b.stopJob(jobID, false, true)

		case jobID := <-b.JobIsRunningCh:
			// zlog.Warn("JobIsRunningCh", jobID)
			r, _ := b.findRun(jobID)
			if r == nil {
				zlog.Error(nil, "JobIsRunningCh on non-existing job", jobID)
				return
			}
			b.setDebugState(jobID, false, false, false, true)
			r.RanAt = time.Now()
			b.HandleSituationFastFunc(*r, JobRunning, nil)
			b.startAndStopRuns()

		case <-b.ChangeJobCh:
			zlog.Warn("ChangeJobCh")

		case e := <-b.AddExecutorCh:
			b.addExector(e)

		case <-b.RemoveExecutorCh:
			zlog.Warn("RemoveExecutorCh")

		case <-b.ChangeExecutorCh:
			zlog.Warn("ChangeExecutorCh")

		case exID := <-b.SetExecutorIsAliveCh:
			e, _ := b.findExecutor(exID)
			e.KeptAliveAt = time.Now()
			zlog.Warn("SetExecutorIsAliveCh!", e.ID)
			b.startAndStopRuns()

		case <-b.refreshCh:
			b.startAndStopRuns()

		case jobID := <-b.endRunCh:
			b.endRun(jobID)
			b.startAndStopRuns()

		case jobID := <-b.removeRunCh:
			b.removeRun(jobID)
			b.startAndStopRuns()

		case <-b.timer.C:
			// zlog.Warn("timer tick")
			b.startAndStopRuns()
		}
	}
}

func (b *Scheduler[I]) removeRun(jobID I) {
	r, i := b.findRun(jobID)
	if i == -1 {
		zlog.Error(nil, "removeRun: job not found", jobID)
		return
	}
	zlog.Warn(r.Job.DebugName, "removeRun", jobID)
	if i != -1 {
		zslice.RemoveAt(&b.runs, i)
	}
}

func (b *Scheduler[I]) endRun(jobID I) {
	_, i := b.findRun(jobID)
	if i == -1 {
		zlog.Error(nil, "clearRun: job not found", jobID)
		return
	}
	b.runs[i].ExecutorID = b.zeroID
	b.runs[i].Removing = false
	b.runs[i].Stopping = false
	b.HandleSituationFastFunc(b.runs[i], JobEnded, nil)
	// b.runs[i].EndedAt = time.Now()
}

func (b *Scheduler[I]) isExecutorAlive(e *Executor[I]) bool {
	if e == nil {
		return false
	}
	if b.ExecutorAliveDuration == 0 {
		return true
	}
	return time.Since(e.KeptAliveAt) < (b.ExecutorAliveDuration*140)/100
}
