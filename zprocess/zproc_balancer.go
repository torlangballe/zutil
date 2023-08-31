package zprocess

import (
	"context"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zslice"
)

type Balancer[I comparable] struct {
	ExecutorAliveDuration                time.Duration
	SimultaneousStarts                   int
	MaximumRunningJobs                   int
	LoadBalanceIfCostDifference          float64
	MinDurationBetweenSimultaneousStarts time.Duration
	KeepJobsBeyondAtEndUntilEnoughSlack  time.Duration
	SlowFuncsTimeout                     time.Duration
	SlowFuncsRetries                     int

	StartJobOnExecutorSlowFunc func(run Run[I], ctx context.Context) error // this function can be slow, like a request. If it errors it can be run again n times until RepeatFuncsWithBackoffUpTo waiting between
	StopJobOnExecutorSlowFunc  func(run Run[I], ctx context.Context) error
	HandleSituationFastFunc    func(run Run[I], s SituationType, err error) // this function must very quickly do something or spawn a go routine

	// The channels are made in NewBalancer()
	StopJobCh           chan I
	RemoveJobCh         chan I
	AddJobCh            chan Job[I]
	ChangeJobCh         chan Job[I]
	JobIsRunningCh      chan I
	AddExecutorCh       chan Executor[I]
	RemoveExecutorCh    chan I
	ChangeExecutorCh    chan Executor[I]
	TouchExecutorCh     chan I
	SetJobsOnExecutorCh chan JobsOnExecutor[I]

	executors []Executor[I]
	runs      []Run[I]
	runCount  int
	zeroID    I
	timer     *time.Timer
	Debug     zmap.LockMap[I, JobDebug]
}

type JobDebug struct {
	Known    time.Time
	Existing time.Time
	Starting time.Time
	Ending   time.Time
	Running  time.Time

	Existed time.Duration
	Started time.Duration
	Ended   time.Duration
	Runned  time.Duration

	JobName      string
	ExecutorName string
}

type Job[I comparable] struct {
	ID        I
	DebugName string
	Duration  time.Duration // How long job should run for. 0 is until stopped.
	Cost      float64       // Cost is how much of executor's spend job uses.
}

type Executor[I comparable] struct {
	ID          I
	Paused      bool
	Spend       float64
	KeptAliveAt time.Time
	DebugName   string
}

type Run[I comparable] struct {
	Job        Job[I]
	ExecutorID I
	Count      int
	Accepted   bool
	RanAt      time.Time
	Started    time.Time
	EndedAt    time.Time
	Removing   bool
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
)

func NewBalancer[I comparable]() *Balancer[I] {
	zlog.Warn("NewBalancer")
	b := &Balancer[I]{}
	b.SlowFuncsRetries = 1

	b.AddJobCh = make(chan Job[I])
	b.RemoveJobCh = make(chan I)
	b.StopJobCh = make(chan I)
	b.ChangeJobCh = make(chan Job[I])
	b.JobIsRunningCh = make(chan I)
	b.AddJobCh = make(chan Job[I])

	b.TouchExecutorCh = make(chan I)
	b.ChangeExecutorCh = make(chan Executor[I])
	b.AddExecutorCh = make(chan Executor[I])
	b.RemoveExecutorCh = make(chan I)
	b.AddExecutorCh = make(chan Executor[I])

	b.ExecutorAliveDuration = time.Second * 10
	b.SimultaneousStarts = 1
	b.MinDurationBetweenSimultaneousStarts = 0
	b.KeepJobsBeyondAtEndUntilEnoughSlack = 0

	return b
}

func (b *Balancer[I]) doTaskRepeatedWithBackoff(task func(ctx context.Context) error, done func(err error)) {
	start := time.Now()
	sleep := b.SlowFuncsTimeout / 10
	ctx, _ := context.WithDeadline(context.Background(), time.Now().Add(b.SlowFuncsTimeout/time.Duration(b.SlowFuncsRetries)))
	for i := 0; i < b.SlowFuncsRetries; i++ {
		taskStart := time.Now()
		err := task(ctx)
		if err == nil {
			done(nil)
			return
		}
		if time.Since(start) >= b.SlowFuncsTimeout {
			done(err)
			break
		}
		since := time.Since(taskStart)
		if since < sleep {
			time.Sleep(sleep - since)
		}
		sleep *= 2
	}
}

func (b *Balancer[I]) findRun(jobID I) (*Run[I], int) {
	for i, r := range b.runs {
		if r.Job.ID == jobID {
			return &b.runs[i], i
		}
	}
	return nil, -1
}

func (b *Balancer[I]) findExecutor(id I) (*Executor[I], int) {
	for i, e := range b.executors {
		if e.ID == id {
			return &b.executors[i], i
		}
	}
	return nil, -1
}

func (b *Balancer[I]) setDebugState(jobID I, existing, starting, ending, running bool) {
	now := time.Now()
	d, got := b.Debug.Get(jobID)
	if !got {
		d.Known = now
	}
	if !d.Existing.IsZero() {
		d.Existed += time.Since(d.Existing)
		d.Existing = time.Time{}
	}
	if !d.Starting.IsZero() {
		d.Started += time.Since(d.Starting)
		d.Starting = time.Time{}
	}
	if !d.Ending.IsZero() {
		d.Ended += time.Since(d.Ending)
		d.Ending = time.Time{}
	}
	if !d.Running.IsZero() {
		d.Runned += time.Since(d.Running)
		d.Running = time.Time{}
	}
	// zlog.Warn("setDebugState:", d.JobName, jobID, existing, starting, running, delta, str)
	if existing {
		d.Existing = now
		d.ExecutorName = ""
	} else if starting {
		d.Starting = now
	} else if running {
		d.Running = now
	} else if ending {
		d.Ending = now
	}
	if !starting && !running && !ending {
		d.ExecutorName = ""
	}
	b.Debug.Set(jobID, d)
}

func (b *Balancer[I]) stopJob(jobID I, remove bool) {
	run, _ := b.findRun(jobID)
	if run == nil {
		zlog.Error(nil, "stop: no run with that id", jobID)
		return
	}
	// zlog.Warn(run.Job.DebugName, "stop") //, zlog.CallingStackString())
	if run.Removing || run.ExecutorID == b.zeroID {
		zlog.Error(nil, "stop: run already removing?", jobID, run.Removing, run.ExecutorID)
		return
	}
	b.setDebugState(jobID, false, false, true, false)
	run.Removing = true
	r := *run
	run.RanAt = time.Time{}
	run.Started = time.Time{}
	run.ExecutorID = b.zeroID
	go b.doTaskRepeatedWithBackoff(func(ctx context.Context) error {
		return b.StopJobOnExecutorSlowFunc(r, ctx)
	}, func(err error) {
		b.setDebugState(jobID, !remove, false, false, false)
		if err != nil {
			go b.HandleSituationFastFunc(*run, RemoveJobFromExecutorFailed, err)
		}
		run.Removing = false
		if remove {
			_, i := b.findRun(jobID)
			if i != -1 {
				zslice.RemoveAt(&b.runs, i)
			}
		}
		b.startAndStopRuns()
	})
}

func (b *Balancer[I]) addJob(job Job[I]) {
	_, i := b.findRun(job.ID)
	zlog.Assert(i == -1)

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

func (b *Balancer[I]) runnableExecutorIDs() map[I]bool {
	m := map[I]bool{}
	for _, e := range b.executors {
		// zlog.Warn("runnableExecutorIDs:", e.Paused, e.KeptAliveAt, b.IsExecutorAlive(&e))
		if !e.Paused && b.IsExecutorAlive(&e) {
			m[e.ID] = true
		}
	}
	return m
}

func (b *Balancer[I]) hasUnrunJobs() bool {
	for _, r := range b.runs {
		if r.ExecutorID == b.zeroID || r.Removing || r.Started.IsZero() {
			return true
		}
	}
	return false
}

func (b *Balancer[I]) maybeStopJobOnPaused(run Run[I]) bool {
	if b.hasUnrunJobs() {
		if b.KeepJobsBeyondAtEndUntilEnoughSlack == 0 {
			return false
		}

	}
	b.stopJob(run.Job.ID, false)
	return true
}

func (b *Balancer[I]) isJobOverdueToQuit(run Run[I]) (quit, over bool) {
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

func (b *Balancer[I]) startAndStopRuns() bool {
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
			if e == nil || quit || !b.IsExecutorAlive(e) {
				// zlog.Warn("startAndStopRuns stop:", r.Job.ID, quit, b.IsExecutorAlive(e))
				b.stopJob(r.Job.ID, false)
				continue
			}
			if e.Paused || over {
				if !paused {
					paused = b.maybeStopJobOnPaused(r)
				}
			}
			continue
		}
		// zlog.Warn("startAndStopRuns", r.Removing, r.Started, r.Job.ID, r.ExecutorID)
		if r.ExecutorID == b.zeroID && !r.Removing && r.Started.IsZero() {
			if oldestRun == nil || r.EndedAt.Sub(oldestRun.EndedAt) > 0 {
				oldestRun = &b.runs[i]
			}
		}
	}
	if oldestRun != nil {
		b.startJob(oldestRun)
	}
	var nextTimerDur time.Duration
	var hasNext bool

	// zlog.Warn("set timer?")
	for _, r := range b.runs {
		if r.Job.Duration == 0 {
			continue
		}
		if !r.RanAt.IsZero() {
			// zlog.Warn("set timer has run")
			if !hasNext || nextTimerDur > r.Job.Duration {
				nextTimerDur = r.Job.Duration
				hasNext = true
			}
		}
	}
	if hasNext {
		// zlog.Warn("SetNextTimer:", nextTimerDur)
		zlog.Assert(nextTimerDur > 0)
		b.timer.Stop()
		b.timer.Reset(nextTimerDur)
	}
	return false
}

type capacity struct {
	load               float64
	startingCount      int
	mostRecentStarting time.Time //!!!!!!!!!!!!!! use this to not run 2 jobs on same worker after each other!
}

func (b *Balancer[I]) calculateLoadOfUsableExecutors() map[I]capacity {
	m := map[I]capacity{}
	runnableEx := b.runnableExecutorIDs()
	for _, e := range b.executors {
		if !runnableEx[e.ID] {
			continue
		}
		m[e.ID] = capacity{}
	}
	// zlog.Warn("calculateLoadOfUsableExecutors:", runnableEx)
	for _, r := range b.runs {
		if !runnableEx[r.ExecutorID] {
			continue
		}
		c := m[r.ExecutorID]
		if !r.Removing {
			if !r.Started.IsZero() {
				c.load += r.Job.Cost
				if r.RanAt.IsZero() {
					c.startingCount++
					if r.Started.Sub(c.mostRecentStarting) > 0 {
						c.mostRecentStarting = r.Started
					}
				}
			}
			m[r.ExecutorID] = c
		}
	}
	return m
}

func (b *Balancer[I]) startJob(run *Run[I]) {
	var bestExID I
	var bestStartingCount = -1
	var bestCapacity float64
	m := b.calculateLoadOfUsableExecutors()
	// zlog.Warn("startJob1?:", run.Job.ID, len(m))
	for exID, cap := range m {
		e, _ := b.findExecutor(exID)
		exCap := e.Spend - cap.load
		if exCap < run.Job.Cost {
			continue
		}
		// zlog.Warn("startJob?:", exID, zlog.Full(cap), b.SimultaneousStarts, b.MinDurationBetweenSimultaneousStarts)
		if cap.startingCount >= b.SimultaneousStarts {
			continue
		}
		if cap.startingCount > 0 && b.SimultaneousStarts > 1 {
			if time.Since(cap.mostRecentStarting) < b.MinDurationBetweenSimultaneousStarts {
				continue
			}
		}
		if bestStartingCount == -1 || cap.startingCount < bestStartingCount || exCap > bestCapacity {
			// above: We prioritize number of current starting over capacity
			bestCapacity = exCap
			bestExID = exID
			bestStartingCount = cap.startingCount
		}
	}
	if bestExID == b.zeroID {
		go b.HandleSituationFastFunc(*run, NoWorkersToRunJob, zlog.NewError("job:", run.Job.ID))
		return
	}
	// zlog.Warn("startJob1?:", run.Job.ID)
	b.setDebugState(run.Job.ID, false, true, false, false)
	run.Started = time.Now()
	run.ExecutorID = bestExID
	d, _ := b.Debug.Get(run.Job.ID)
	e, _ := b.findExecutor(run.ExecutorID)
	d.ExecutorName = e.DebugName
	b.Debug.Set(run.Job.ID, d)
	go b.doTaskRepeatedWithBackoff(func(ctx context.Context) error {
		// zlog.Warn(run.Job.DebugName, "start")
		return b.StartJobOnExecutorSlowFunc(*run, ctx)
	}, func(err error) {
		if err != nil {
			zlog.Warn(run.Job.ID, "started end err", err)
			run.RanAt = time.Time{}
			run.Started = time.Time{}
			run.ExecutorID = b.zeroID
			go b.HandleSituationFastFunc(*run, ErrorStartingJob, err)
		}
	})
}

func (b *Balancer[I]) addExector(e Executor[I]) {
	zlog.Warn("addExecutor")
	b.executors = append(b.executors, e)
	b.startAndStopRuns()
}

func (b *Balancer[I]) Start() {
	b.timer = time.NewTimer(0)
	for {
		select {
		case j := <-b.AddJobCh:
			b.addJob(j)

		case jobID := <-b.RemoveJobCh:
			zlog.Warn("RemoveJobCh", jobID)
			b.stopJob(jobID, true)

		case jobID := <-b.StopJobCh:
			zlog.Warn("StopJobCh", jobID)
			b.stopJob(jobID, false)

		case jobID := <-b.JobIsRunningCh:
			// zlog.Warn("JobIsRunningCh", jobID)
			r, _ := b.findRun(jobID)
			zlog.Assert(r != nil)
			b.setDebugState(jobID, false, false, false, true)
			r.RanAt = time.Now()
			b.startAndStopRuns()

		case <-b.ChangeJobCh:
			zlog.Warn("ChangeJobCh")

		case e := <-b.AddExecutorCh:
			b.addExector(e)

		case <-b.RemoveExecutorCh:
			zlog.Warn("RemoveExecutorCh")

		case <-b.ChangeExecutorCh:
			zlog.Warn("ChangeExecutorCh")

		case exID := <-b.TouchExecutorCh:
			e, _ := b.findExecutor(exID)
			e.KeptAliveAt = time.Now()
			zlog.Warn("Touch!", e.ID)
			b.startAndStopRuns()

		case <-b.timer.C:
			// zlog.Warn("timer tick")
			b.startAndStopRuns()
		}
	}
}

func (b *Balancer[I]) IsExecutorAlive(e *Executor[I]) bool {
	if b.ExecutorAliveDuration == 0 {
		return true
	}
	return time.Since(e.KeptAliveAt) < (b.ExecutorAliveDuration*140)/100
}
