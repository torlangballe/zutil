package zprocess

import (
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
)

type Balancer[I comparable] struct {
	ExecutorAliveDuration                time.Duration
	SimultaneousStarts                   int
	MaximumRunningJobs                   int
	MinDurationBetweenSimultaneousStarts time.Duration
	KeepJobsBeyondAtEndUntilEnoughSlack  time.Duration
	RateLimit                            time.Duration

	StartJobOnExecutorSlowFunc func(jobID, executorID I) error
	StopJobOnExecutorSlowFunc  func(jobID, executorID I) error
	HandleSituationFastFunc    func(jobID, executorID I, s SituationType, err error) // this function must very quicly do something or spawn a go routine

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
}

type Job[I comparable] struct {
	ID        I
	DebugName string
	Duration  time.Duration
	Cost      float64
}

type Executor[I comparable] struct {
	ID          I
	Paused      bool
	Spend       float64
	KeptAliveAt time.Time
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

func (b *Balancer[I]) doTaskRateLimited(task func() error, done func(err error)) {
	sleep := time.Millisecond * 50
	for {
		start := time.Now()
		err := task()
		if err == nil {
			done(nil)
			return
		}
		if sleep >= b.RateLimit {
			done(err)
			break
		}
		since := time.Since(start)
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

func (b *Balancer[I]) stopJob(jobID I, remove bool) {
	zlog.Warn(jobID, "stop") //, zlog.CallingStackString())
	run, _ := b.findRun(jobID)
	if run == nil {
		zlog.Error(nil, "stop: no run with that id", jobID)
		return
	}
	if run.Removing || run.ExecutorID == b.zeroID {
		zlog.Error(nil, "stop: run already removing?", jobID, run.Removing, run.ExecutorID)
		return
	}
	run.Removing = true
	exID := run.ExecutorID
	run.RanAt = time.Time{}
	run.Started = time.Time{}
	run.ExecutorID = b.zeroID
	go b.doTaskRateLimited(func() error {
		return b.StopJobOnExecutorSlowFunc(run.Job.ID, exID)
	}, func(err error) {
		if err != nil {
			go b.HandleSituationFastFunc(run.Job.ID, run.ExecutorID, RemoveJobFromExecutorFailed, err)
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
		return true
	}
	return false
}

type capacity struct {
	load               float64
	startingCount      int
	mostRecentStarting time.Time
}

func (b *Balancer[I]) calculateLoadOfUsableExecutors() map[I]capacity {
	m := map[I]capacity{}
	runnableEx := b.runnableExecutorIDs()
	// zlog.Warn("calculateLoadOfUsableExecutors:", runnableEx)
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
	var bestCapacity float64
	m := b.calculateLoadOfUsableExecutors()
	// zlog.Warn("startJob1?:", run.Job.ID)
	for exID, cap := range m {
		// zlog.Warn("startJob?:", exID, zlog.Full(cap), b.SimultaneousStarts, b.MinDurationBetweenSimultaneousStarts)
		if cap.startingCount >= b.SimultaneousStarts {
			continue
		}
		if cap.startingCount > 0 && b.SimultaneousStarts > 1 {
			if time.Since(cap.mostRecentStarting) < b.MinDurationBetweenSimultaneousStarts {
				continue
			}
		}
		e, _ := b.findExecutor(exID)
		cap := e.Spend - cap.load
		if cap > bestCapacity {
			bestCapacity = cap
			bestExID = exID
		}
	}
	if bestExID == b.zeroID {
		go b.HandleSituationFastFunc(run.Job.ID, run.ExecutorID, NoWorkersToRunJob, zlog.NewError("job:", run.Job.ID))
		return
	}
	run.Started = time.Now()
	run.ExecutorID = bestExID
	go b.doTaskRateLimited(func() error {
		zlog.Warn(run.Job.ID, "start")
		return b.StartJobOnExecutorSlowFunc(run.Job.ID, run.ExecutorID)
	}, func(err error) {
		if err != nil {
			zlog.Warn(run.Job.ID, "started end err", err)
			run.RanAt = time.Time{}
			run.Started = time.Time{}
			run.ExecutorID = b.zeroID
			go b.HandleSituationFastFunc(run.Job.ID, run.ExecutorID, ErrorStartingJob, err)
		}
	})
}

func (b *Balancer[I]) addExector(e Executor[I]) {
	zlog.Warn("addExecutor")
	b.executors = append(b.executors, e)
	b.startAndStopRuns()
}

func (b *Balancer[I]) Start() {
	ticker := time.NewTicker(time.Millisecond * 500)
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

		case <-ticker.C:
			// zlog.Warn("second tick")
			b.startAndStopRuns()
		}
	}
	// ticker.Stop()
}

func (b *Balancer[I]) IsExecutorAlive(e *Executor[I]) bool {
	if e.KeptAliveAt.IsZero() {
		return false
	}
	return time.Since(e.KeptAliveAt) < (b.ExecutorAliveDuration*140)/100
}
