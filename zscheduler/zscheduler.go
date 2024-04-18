package zscheduler

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
)

type Setup[I comparable] struct {
	ExecutorAliveDuration                time.Duration                                     // ExecutorAliveDuration is how often an executor needs to say it's alive to be considered operatable. 0 means always alive.
	SimultaneousStarts                   int                                               // SimultaneousStarts is how many jobs can start while anotherone is starting and hasn't reached rnuning state yet. See also MinDurationBetweenSimultaneousStarts.
	MinDurationBetweenSimultaneousStarts time.Duration                                     // MinDurationBetweenSimultaneousStarts is how long to wait to do next start if SimultaneousStarts > 1.
	LoadBalanceIfCostDifference          float64                                           // If LoadBalanceIfCostDifference > 0, once all jobs are running, switch job to an executor with more capacity left if difference > this.
	KeepJobsBeyondAtEndUntilEnoughSlack  time.Duration                                     // If KeepJobsBeyondAtEndUntilEnoughSlack > 0, a job isn't stopped at Duration end if there's other jobs not in run state yet, yet are stopped if they go beyond this duration extra.
	SlowStartJobFuncTimeout              time.Duration                                     // SlowStartJobFuncTimeout is how long starting a job with StartJobOnExecutorFunc can go until timeout.
	SlowStopJobFuncTimeout               time.Duration                                     // SlowStopJobFuncTimeout is like SlowStartJobFuncTimeout/StopJobOnExecutorFunc but for stopping.
	TotalMaxJobCount                     int                                               // The scheduler wont start another job if active jobs >= TotalMaxJobCount.
	JobIsRunningOnSuccessfullStart       bool                                              // Set JobIsRunningOnSuccessfullStart to set a job as running once its start function completes successfully. Otherwise use the JobIsRunningCh channel.
	ChangingJobRestartsIt                bool                                              // If ChangingJobRestartsIt is set, jobs are restarted when changed with ChangeExecutorCh.
	GracePeriodForJobsOnExecutorCh       time.Duration                                     // GracePeriodForJobsOnExecutorCh is amount of slack from start to not stop jobs not reported in executor yet.
	StartJobOnExecutorFunc               func(run Run[I], ctx context.Context) error       `zui:"-"` // StartJobOnExecutorFunc is called to start a job. It is done on a goroutine and is assumed to take a while or time out.
	StopJobOnExecutorFunc                func(run Run[I], ctx context.Context) error       `zui:"-"` // Like StartJobOnExecutorFunc but for stopping.
	HandleSituationFastFunc              func(run Run[I], s SituationType, details string) `zui:"-"` // This function is for handling start/stop/errors and more. Must very quickly do something or spawn a go routine
	StopJobIfSinceMilestoneLessThan      time.Duration                                     // Only stop job if StopJobIfSinceMilestoneLessThan != 0, and time since run.MilestoneAt is less than it, up to KeepJobsBeyondAtEndUntilEnoughSlack (which also must be set)
	// MinimumTimeBetweenSpecificJobStarts  time.Duration
}

// A *Scheduler* starts *Job*s on *Executor*s, trying to balance the workload
// Each Job has a *Cost*, and each executor a *CostCapacity*.
// Jobs can have a Duration or go until stopped.
// The scheduler assumes jobs take a considerable time to start and end,
// and can get congested if too many are starting at once, so has
// SimultaneousStarts and MinDurationBetweenSimultaneousStarts parameters.
// With these constraints, priority is to start jobs as soon as possible on any executor with enough capacity.
// All changes to a Scheduler are done through channels
type Scheduler[I comparable] struct {
	// The channels are made in NewScheduler()
	StopJobCh               chan I                 // Write a Job ID to StopJobCh to stop the job.
	RemoveJobCh             chan I                 // Write a Job ID to RemoveJobCh to stop and remove the job.
	AddJobCh                chan Job[I]            // Write a Job to AddJobCh to add a job. It will be started when and how possible.
	ChangeJobCh             chan Job[I]            // Write  a Job with existing ID to ChangeJobCh to change it. It will be restarted if ChangingJobRestartsIt is true.
	JobIsRunningCh          chan I                 // Write a JobID to JobIsRunningCh to flag it as now running. Not needed if JobIsRunningOnSuccessfullStart true.
	AddExecutorCh           chan Executor[I]       // Write an Executor to AddExecutorCh to add an executor. Jobs will start/balance onto it as needed.
	RemoveExecutorCh        chan I                 //  Write an executor ID to RemoveExecutorCh to remove it. Jobs on it will immediately be stopped and restarted.
	ChangeExecutorCh        chan Executor[I]       // Wriite an executor with existing ID to ChangeExecutorCh to change it. Jobs on it will be restarted if anything but DebugName is changed.
	FlushExecutorJobsCh     chan I                 // Write an executor ID to FlushExecutorJobsCh to restart all jobs on it.
	SetExecutorIsAliveCh    chan I                 // Write an executor ID to SetExecutorIsAliveCh to keep it alive at least with frequency ExecutorAliveDuration.
	SetJobsOnExecutorCh     chan JobsOnExecutor[I] // Write a list of job ids and executor id to SetJobsOnExecutorCh to update what is running on the executor. Since the executor might crash, this keeps jobs in sync.
	SetTotalMaxJobCountCh   chan int               // Change TotalMaxJobCount and refresh scheduler based on that.
	SetJobHasMilestoneNowCh chan I                 // Sets job's run's milestone to now. See StopJobIfSinceMilestoneLessThan.
	SetJobHasErrorCh        chan I                 // Sets the run's ErrorAt to now

	setup       Setup[I]
	refreshCh   chan struct{} // Writing an empty struct to refreshCh calls stopAndStartJobs(), but without recursion (and possible locking).
	endRunCh    chan I        // endRunCh sets a run to after stopped status
	removeRunCh chan I        // removeRunCh does local remove of a run

	executors []Executor[I]
	runs      []Run[I]
	zeroID    I
	timer     *time.Timer
	Debug     zmap.LockMap[I, JobDebug]
	timerOn   bool
	stopped   bool
	started   bool
}

type Job[I comparable] struct {
	ID           I
	DebugName    string
	Duration     time.Duration // How long job should run for. 0 is until stopped.
	Cost         float64       // Cost is how much of an executor's CostCapacity the job uses.
	changedCount int           // changedCount is an incremented when job changes. Must be flushed then.
}

type Executor[I comparable] struct {
	ID           I
	Paused       bool
	CostCapacity float64
	KeptAliveAt  time.Time
	DebugName    string
	SettingsHash int64 // other settings for executor, if changed cause restart
	changedCount int   // changedCount is an incremented when executor changes. Must be flushed then
}

type Run[I comparable] struct {
	Job         Job[I]    `zui:"flatten"`
	ExecutorID  I         `zui:"title:ExID"`
	Count       int       `zui:"allowempty"`
	StartedAt   time.Time `zui:"allowempty"`
	RanAt       time.Time `zui:"allowempty"`
	StoppedAt   time.Time `zui:"allowempty"`
	Removing    bool      `zui:"allowempty"`
	Stopping    bool      `zui:"allowempty"`
	MilestoneAt time.Time `zui:"allowempty"` // MilestoneAt is a time a significant sub-task was achieved. See StopJobIfSinceMilestoneLessThan above.
	ErrorAt     time.Time `zui:"allowempty"` // ErrorAt is last time an error occured on this job/run. Used to de-prioritize jobs with recent errors when starting new jobs.

	starting             bool
	executorChangedCount int
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
	ExecutorHasExpired          SituationType = "executor is no longer alive"
	MaximumJobsReached          SituationType = "maximum jobs reached"
	SchedulerFinishedStopping   SituationType = "scheduler finished stopping"
	JobStarted                  SituationType = "job started"
	JobRunning                  SituationType = "job running"
	JobStopped                  SituationType = "job stopped"
	JobEnded                    SituationType = "job ended"
)

func NewScheduler[I comparable]() *Scheduler[I] {
	s := &Scheduler[I]{}
	s.AddJobCh = make(chan Job[I])
	s.RemoveJobCh = make(chan I)
	s.StopJobCh = make(chan I)
	s.ChangeJobCh = make(chan Job[I])
	s.JobIsRunningCh = make(chan I)
	s.AddJobCh = make(chan Job[I])
	s.SetExecutorIsAliveCh = make(chan I)
	s.SetJobsOnExecutorCh = make(chan JobsOnExecutor[I])
	s.SetTotalMaxJobCountCh = make(chan int)
	s.SetJobHasMilestoneNowCh = make(chan I)
	s.SetJobHasErrorCh = make(chan I)
	s.ChangeExecutorCh = make(chan Executor[I])
	s.FlushExecutorJobsCh = make(chan I)
	s.AddExecutorCh = make(chan Executor[I])
	s.RemoveExecutorCh = make(chan I)
	s.AddExecutorCh = make(chan Executor[I])
	s.refreshCh = make(chan struct{}) // 10
	s.endRunCh = make(chan I)
	return s
}

func (s *Scheduler[I]) Init(setup Setup[I]) {
	s.setup = setup
	s.timer = time.NewTimer(0)
	go s.selectLoop()
}

var reason string

func (s *Scheduler[I]) selectLoop() {
	for {
		var timer *ztimer.Timer
		reason = ""
		timer = ztimer.StartIn(2, func() {
			if reason != "" {
				zlog.Info("************** loop hung!!!", reason)
			}
		})
		select {
		case j := <-s.AddJobCh:
			reason = "AddJobCh"
			if !s.stopped {
				s.addJob(j, true)
			}

		case jobID := <-s.RemoveJobCh:
			reason = zstr.Spaced("RemoveJobCh", jobID)
			s.stopJob(jobID, true, true, true, reason)

		case jobID := <-s.StopJobCh:
			reason = "StopJobCh"
			s.stopJob(jobID, false, true, true, reason)

		case jobID := <-s.JobIsRunningCh:
			reason = "JobIsRunningCh"
			s.setJobRunning(jobID)

		case job := <-s.ChangeJobCh:
			reason = "ChangeJobCh"
			s.changeJob(job)

		case e := <-s.AddExecutorCh:
			reason = "AddExCh"
			s.addExecutor(e)

		case exID := <-s.RemoveExecutorCh:
			// zlog.Warn("RemoveExecutorCh", zlog.CallingStackString())
			reason = "RemoveExCh"
			s.removeExecutor(exID)

		case exID := <-s.FlushExecutorJobsCh:
			reason = "FlushExecutorJobsCh"
			fe, _ := s.findExecutor(exID)
			if fe != nil {
				// zlog.Warn("FLUSH!!!")
				fe.changedCount++
				s.startAndStopRuns()
			}

		case ex := <-s.ChangeExecutorCh:
			reason = "ChangeExecutorCh"
			s.changeExecutor(ex)

		case jobsOnExe := <-s.SetJobsOnExecutorCh:
			reason = "SetJobsOnExecutorCh"
			s.purgeStartedJobsNotInList(jobsOnExe)

		case max := <-s.SetTotalMaxJobCountCh:
			reason = "SetTotalMaxJobCountCh"
			s.setup.TotalMaxJobCount = max
			s.startAndStopRuns()

		case jobID := <-s.SetJobHasMilestoneNowCh:
			s.setMilestoneForRun(jobID, time.Now())

		case jobID := <-s.SetJobHasErrorCh:
			run, _ := s.findRun(jobID)
			if run == nil {
				zlog.Error("SetJobHasErrorCh: no run for", jobID)
				return
			}
			run.ErrorAt = time.Now()

		case exID := <-s.SetExecutorIsAliveCh:
			reason = "SetExecutorIsAliveCh"
			e, _ := s.findExecutor(exID)
			if e == nil {
				zlog.Error("SetExecutorIsAlive: none with id:", exID)
			} else {
				e.KeptAliveAt = time.Now()
				s.startAndStopRuns()
			}

		case <-s.refreshCh:
			// zlog.Warn("Refresh")
			reason = "refreshCh"
			s.startAndStopRuns()

		case jobID := <-s.endRunCh:
			reason = "endRunCh"
			// zlog.Warn("endRunCh pushed:", jobID)
			s.endRun(jobID)

		case jobID := <-s.removeRunCh:
			reason = "removeRunCh"
			s.removeRun(jobID)
			s.startAndStopRuns()

		case <-s.timer.C:
			// zlog.Warn("tick")
			s.timerOn = false
			s.startAndStopRuns()
		}
		timer.Stop()
	}
}

func (s *Scheduler[I]) setMilestoneForRun(jobID I, at time.Time) {
	run, _ := s.findRun(jobID)
	if run == nil {
		zlog.Error("Scheduler setMilestoneForRun, not run with that job id:", jobID)
		return
	}
	run.MilestoneAt = at
	s.startAndStopRuns()
}

func (s *Scheduler[I]) Start() {
	s.started = true
	s.refreshCh <- struct{}{} // important to do this, or we could be running startAndStopRuns() in a different goroutine
}

func DefaultSetup[I comparable]() Setup[I] {
	var s Setup[I]

	s.SlowStartJobFuncTimeout = time.Second * 10
	s.SlowStopJobFuncTimeout = time.Second * 10
	s.TotalMaxJobCount = -1
	s.ExecutorAliveDuration = time.Second * 10
	s.SimultaneousStarts = 1
	s.MinDurationBetweenSimultaneousStarts = 0
	s.KeepJobsBeyondAtEndUntilEnoughSlack = 0
	s.HandleSituationFastFunc = func(run Run[I], s SituationType, details string) {}
	s.JobIsRunningOnSuccessfullStart = false
	return s
}

func (s *Scheduler[I]) findRun(jobID I) (*Run[I], int) {
	for i, r := range s.runs {
		if r.Job.ID == jobID {
			return &s.runs[i], i
		}
	}
	return nil, -1
}

func (s *Scheduler[I]) findExecutor(id I) (*Executor[I], int) {
	for i, e := range s.executors {
		if e.ID == id {
			return &s.executors[i], i
		}
	}
	return nil, -1
}

func (s *Scheduler[I]) stopJob(jobID I, remove, outsideRequest, refresh bool, reason string) {
	// if s.stopped {
	// }

	now := time.Now()
	run, _ := s.findRun(jobID)
	// zlog.Warn("StopJob:", jobID, run != nil)
	if run == nil {
		if s.stopped {
			return
		}
		zlog.Error("Scheduler stop: no run with that id", jobID, "reason to stop was:", reason, zlog.CallingStackString())
		zlog.Assert(outsideRequest)
		return
	}
	if run.ExecutorID == s.zeroID {
		// if !outsideRequest {
		zlog.Warn("stopJob: not running", jobID)
		// }
	}
	// zlog.Warn("stopJob", jobID, run.Stopping, remove, outsideRequest, zlog.CallingStackString())
	defer func() {
		// if s.stopped {
		// zlog.Warn("stopJob refresh?", jobID, refresh)
		// }
		if refresh {
			s.startStopViaChannelNonBlocking() // pushNonBlockingToChannel(s.refreshCh, struct{}{})
			// if s.stopped {
			// 	zlog.Warn("stopJob refresh after push", jobID)
			// }
		}
	}()
	// zlog.Warn("stopJob2?", jobID, run.ExecutorID)
	// zlog.Warn("stopJob", jobID, run.Stopping, remove, outsideRequest) //, zlog.CallingStackString())
	if run.Stopping {
		// zlog.Warn("Bailing stopJob on stopping already", zlog.Full(run)) //, zlog.CallingStackString())
		if remove && !run.Removing {
			run.Removing = true
			// zlog.Warn("Upgrading stop to remove from outside stop+remove", run.Job.DebugName)
		}
		return
	}
	// if s.stopped {
	// 	zlog.Warn("stopJob", run.StartedAt, "remove:", remove, "removing:", run.Removing, run.Job.DebugName, run.ExecutorID) //, zlog.CallingStackString())
	// }
	s.setDebugState(jobID, false, false, true, false)
	run.Stopping = true
	run.StoppedAt = now
	run.Removing = remove
	run.RanAt = time.Time{}
	run.StartedAt = time.Time{}
	run.starting = false
	r := *run
	// zlog.Warn("stopJob handleSit:", r.ExecutorID, r.Job.ID, r.ExecutorID)
	s.setup.HandleSituationFastFunc(r, JobStopped, reason)
	// run.ExecutorID = s.zeroIDe
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(s.setup.SlowStopJobFuncTimeout))
	// zlog.Warn("stopJob", run.Job.DebugName, run.ExecutorID, refresh, remove, outsideRequest, "reason:", reason, run) //, zlog.CallingStackString())
	go func() {
		err := s.setup.StopJobOnExecutorFunc(r, ctx)
		s.setDebugState(jobID, !remove, false, false, false)
		// if s.stopped {
		// rr, _ := s.findRun(jobID)
		// zlog.Warn("stopJob Ended", r.Job.DebugName, err, r.Removing, rr.StartedAt, len(s.runs))
		// }
		if err != nil {
			s.setup.HandleSituationFastFunc(r, RemoveJobFromExecutorFailed, err.Error())
			rr, _ := s.findRun(jobID)
			if rr != nil {
				rr.ErrorAt = time.Now()
			}
		}
		s.endRunCh <- jobID
	}()
	cancel()
}

func (s *Scheduler[I]) addJob(job Job[I], outsideRequest bool) {
	// zlog.Warn("AddJob1:", job.DebugName)
	_, i := s.findRun(job.ID)
	if i != -1 {
		// zlog.Error("adding job when existing", job.DebugName, i)
		zlog.Assert(outsideRequest)
		return
	}
	s.setDebugState(job.ID, true, false, false, false)
	d, _ := s.Debug.Get(job.ID)
	d.JobName = job.DebugName
	s.Debug.Set(job.ID, d)

	var run Run[I]
	run.Job = job
	run.Count = 0
	// zlog.Warn("AddJob:", job.DebugName)
	s.runs = append(s.runs, run)
	s.startAndStopRuns()
}

func (s *Scheduler[I]) runnableExecutorIDs() map[I]bool {
	m := map[I]bool{}
	for _, e := range s.executors {
		// zlog.Warn("runnableExecutorIDs?:", s.isExecutorAlive(&e))
		if !e.Paused && s.isExecutorAlive(&e) {
			m[e.ID] = true
		}
	}
	return m
}

func (s *Scheduler[I]) hasUnrunJobs() bool {
	for _, r := range s.runs {
		if r.ExecutorID == s.zeroID || r.Stopping || r.Removing || r.StartedAt.IsZero() || r.RanAt.IsZero() {
			return true
		}
	}
	return false
}

func (s *Scheduler[I]) shouldStopJob(run Run[I], e *Executor[I], caps map[I]capacity) (stop bool, reason string) {
	// zlog.Warn("shouldStopJob", run.Job.ID, "@", run.ExecutorID, run.Stopping, e == nil, run.StartedAt.IsZero(), run.RanAt.IsZero())
	if run.Stopping {
		// zlog.Warn("Hear1")
		return false, "stopping already"
	}
	alive := s.isExecutorAlive(e)
	if !alive {
		// s.setup.HandleSituationFastFunc(run, ExecutorHasExpired, "")
		return true, "not alive executor"
	}
	if e == nil {
		// zlog.Warn("Hear2", run.Stopping, run.StartedAt.IsZero(), run.RanAt)
		if !run.StartedAt.IsZero() {
			return true, "No executor, and StartAt is not zero"
		}
		if !run.RanAt.IsZero() {
			return true, "No executor, and RanAt is not zero"
		}
		if !alive {
			return true, "No executor, and not alive"
		}
		return false, "No executor, no need to stop job"
	}
	if !alive {
		return true, "Not alive"
	}
	if e.changedCount != run.executorChangedCount {
		// zlog.Warn("shouldStopJob changed:", e.changedCount, run.executorChangedCount)
		return true, zstr.Spaced("executor changeCount changed:", e.changedCount, run.executorChangedCount)
	}
	// zlog.Warn("shouldStopJob2", run.Job.ID, "@", run.ExecutorID, s.isExecutorAlive(e), run.Stopping, e == nil, run.StartedAt.IsZero(), run.RanAt.IsZero())
	var existingCap, unrunCost float64
	var unrunName string
	for _, r := range s.runs {
		if r.Job.ID != run.Job.ID && r.ExecutorID == s.zeroID || r.Stopping || r.Removing || r.StartedAt.IsZero() || r.RanAt.IsZero() {
			unrunCost += r.Job.Cost
			unrunName = r.Job.DebugName
		}
	}
	for _, cap := range caps {
		// if id == e.ID {
		// 	continue
		// }
		// zlog.Warn("Cap:", eID, cap.capacity, cap.load)
		existingCap += cap.spare()
	}
	left := existingCap - unrunCost
	var needsMilestone bool
	var extraStr string
	sinceRun := time.Since(run.RanAt)
	hasSlack := (s.setup.KeepJobsBeyondAtEndUntilEnoughSlack != 0 && sinceRun <= run.Job.Duration+s.setup.KeepJobsBeyondAtEndUntilEnoughSlack)
	if hasSlack && s.setup.StopJobIfSinceMilestoneLessThan != 0 {
		extraStr = ", and near milestone"
		if run.MilestoneAt.IsZero() || time.Since(run.MilestoneAt) > s.setup.StopJobIfSinceMilestoneLessThan {
			needsMilestone = true
		}
	}
	if e.Paused {
		// zlog.Warn("shouldStopJob paused:", caps, existingCap, unrunCost, run.Job.ID, s.setup.KeepJobsBeyondAtEndUntilEnoughSlack, left, run.Job.Cost)
		if s.setup.KeepJobsBeyondAtEndUntilEnoughSlack == 0 {
			return true, "paused and no KeepJobsBeyondAtEndUntilEnoughSlack"
		}
		if needsMilestone {
			return false, ""
		}
		// zlog.Warn("Stop???", run.Job.ID, run.ExecutorID, left, run.Job.Cost, unrunCost)
		if unrunCost == 0 {
			// if left >= run.Job.Cost && unrunCost == 0 {
			return true, zstr.Spaced("paused, and left > cost", left, run.Job.Cost)
		}
		if left <= 0 {
			return true, zstr.Spaced("paused, and left <= 0", left, run.Job.Cost)
		}
		if run.RanAt.IsZero() {
			return true, "paused, and RanAt is zero"
		}
		if time.Since(run.RanAt.Add(run.Job.Duration)) > s.setup.KeepJobsBeyondAtEndUntilEnoughSlack {
			return true, zstr.Spaced("paused, and since RanAt+dur > KeepJobsBeyondAtEndUntilEnoughSlack", time.Since(run.RanAt), ">", s.setup.KeepJobsBeyondAtEndUntilEnoughSlack)
		}
		return false, "" // we don't do other stuff
	}

	if run.RanAt.IsZero() {
		return false, ""
	}
	if run.Job.Duration == 0 {
		return false, ""
	}
	if sinceRun > run.Job.Duration {
		if s.setup.KeepJobsBeyondAtEndUntilEnoughSlack == 0 {
			return true, zstr.Spaced("job duration without slack over", time.Since(run.RanAt), run.Job.Duration)
		}
		if left > run.Job.Cost && unrunCost == 0 {
			if needsMilestone {
				// zlog.Warn("Ready to stop job with capacity, but waiting for milestone", run.Job.DebugName)
			} else {
				return true, zstr.Spaced("job duration with slack over and has capacity"+extraStr, unrunCost, time.Since(run.RanAt), run.Job.Duration, left, run.Job.Cost)
			}
		}
		if !hasSlack {
			return true, zstr.Spaced("job duration with slack and not enough capacity over, stopping anyway", unrunCost, time.Since(run.RanAt), run.Job.Duration, "left:", left, s.setup.KeepJobsBeyondAtEndUntilEnoughSlack)
		}
		str := zstr.Spaced("job duration with slack and not enough capacity still has slack, not stopping yet", time.Since(run.RanAt), run.Job.Duration, "left:", left, "cost:", run.Job.Cost, "unrun:", unrunCost, s.setup.KeepJobsBeyondAtEndUntilEnoughSlack, "urn:", unrunName)
		// zlog.Info("Job not stopped:", run.Job.DebugName, "@", e.DebugName, str)
		return false, str
	}
	// zlog.Warn("Here!", run.Job.DebugName, time.Since(run.RanAt))
	return false, "No reason to stop"
}

// isBetterRunCandidate prioritzes being run longest ago, if not having an error and other does, or having error longer ago.
func isBetterRunCandidate[I comparable](is, other *Run[I]) bool {
	// zlog.Warn("isBetterRunCandidate:", is.Job.ID, is.ErrorAt, other.Job.ID, other.ErrorAt)
	if !is.ErrorAt.IsZero() && other.ErrorAt.IsZero() {
		return false
	}
	if is.ErrorAt.IsZero() && !other.ErrorAt.IsZero() {
		return true
	}
	if !is.ErrorAt.IsZero() && !other.ErrorAt.IsZero() {
		return is.ErrorAt.Before(other.ErrorAt)
	}
	if is.StoppedAt.Before(other.StoppedAt) {
		return true
	}
	return false
}

var ssLock sync.Mutex
var ssCount int

func (s *Scheduler[I]) startAndStopRuns() {
	ssCount++
	// zlog.Warn("startAndStopRuns", ssCount)
	// defer zlog.Warn("startAndStopRuns END", ssCount)
	if !ssLock.TryLock() {
		panic("startAndStopRuns already running!")
	}
	defer ssLock.Unlock()
	// defer zlog.Warn("startAndStopRuns done")
	if !s.started {
		return
	}
	for {
		// zlog.Warn("startAndStopRuns", s.started, len(s.executors), len(s.runs)) //, zlog.CallingStackString())
		var oldestRun *Run[I]
		var bestBalanceJobID I
		var bestLeft float64
		var bestExID I
		var bestRunTime time.Time
		var jobStopped bool
		var active int
		capacities := s.calculateLoadOfUsableExecutors()
		hasUnrun := s.hasUnrunJobs()
		for i, r := range s.runs {
			e, _ := s.findExecutor(r.ExecutorID)
			// if s.stopped {
			// zlog.Warn(i, "FoundExe:", r.ExecutorID, e != nil)
			// }
			stop, stopReason := s.shouldStopJob(r, e, capacities)
			// if stop {
			// zlog.Warn("ShouldStop?:", stop, r.Job.ID, r.Stopping, r.ExecutorID, e != nil, capacities, stopReason)
			// }
			if stop {
				jobStopped = true
				s.stopJob(r.Job.ID, s.stopped, false, true, stopReason)
				break
			}
			if s.stopped {
				continue
			}
			if !(r.StartedAt.IsZero() && r.RanAt.IsZero()) {
				active++
			}
			// zlog.Warn(i, "loop:", r.Job.ID, r.ErrorAt, r.ExecutorID, r.Stopping, r.StartedAt.IsZero())
			if r.ExecutorID == s.zeroID && !r.Stopping && r.StartedAt.IsZero() {
				if oldestRun == nil || isBetterRunCandidate[I](&r, oldestRun) {
					// zlog.Warn(i, "set oldestRun:", len(s.runs), oldestRun != nil, s.runs[i].Job.ID, ssCount, r.ErrorAt)
					oldestRun = &s.runs[i]
				}
			}
			if hasUnrun || s.setup.LoadBalanceIfCostDifference == 0 || r.RanAt.IsZero() || r.Stopping {
				continue
			}
			runLeft := capacities[r.ExecutorID].unusedRatio()
			rDiff := s.setup.LoadBalanceIfCostDifference / capacities[r.ExecutorID].capacity
			for exID, cap := range capacities {
				if exID == r.ExecutorID {
					continue
				}
				eLeft := cap.unusedRatio()
				eDiff := s.setup.LoadBalanceIfCostDifference / cap.capacity
				diff := math.Max(rDiff, eDiff)
				// zlog.Warn(r.Job.ID, exID, "Diffs:", eLeft, runLeft, cap.spare())
				// zlog.Warn("startAndStopRuns LB?", capacities[r.ExecutorID].load, r.Job.ID, r.ExecutorID, hasUnrun, runLeft, eLeft, s.LoadBalanceIfCostDifference)
				if eLeft-runLeft < diff { //s.LoadBalanceIfCostDifference {
					continue
				}
				if eLeft > bestLeft && eLeft >= diff { //s.LoadBalanceIfCostDifference {
					bestLeft = eLeft
					bestExID = exID
					if r.Job.Cost < cap.spare() && (bestRunTime.IsZero() || r.RanAt.Sub(bestRunTime) < 0) {
						// zlog.Warn("Balance at:", r.Job.ID, eLeft, runLeft)
						if s.setup.StopJobIfSinceMilestoneLessThan != 0 && !r.MilestoneAt.IsZero() && time.Since(r.MilestoneAt) > s.setup.StopJobIfSinceMilestoneLessThan {
							zlog.Info("zscheduler:Not adding job to bestBalance since not near milestone:", r.Job.DebugName, r.MilestoneAt)
						} else {
							bestBalanceJobID = r.Job.ID
							bestRunTime = r.RanAt
						}
					}
				}
			}
		}
		// zlog.Warn("startAndStopRuns3", jobStopped, oldestRun != nil)
		if jobStopped {
			continue
		}
		if !s.stopped {
			if oldestRun != nil {
				// zlog.Warn("oldestRun?:", oldestRun != nil, ssCount, s.setup.TotalMaxJobCount, active)
				if s.setup.TotalMaxJobCount != -1 && active >= s.setup.TotalMaxJobCount {
					s.setup.HandleSituationFastFunc(*oldestRun, MaximumJobsReached, zstr.Spaced(active, ">", s.setup.TotalMaxJobCount))
				} else {
					// zlog.Warn("StartJob!")
					if !s.startJob(oldestRun, capacities) {
						// zlog.Warn("StartJob didn't start, refresh")
						s.startStopViaChannelNonBlocking() //						pushNonBlockingToChannel(s.refreshCh, struct{}{})
					}
					return // we don't need to set a timer if we call startJob
				}
			}
			// zlog.Warn("bestBalanceJobID:", bestBalanceJobID)
			if oldestRun == nil && bestBalanceJobID != s.zeroID {
				zdebug.Consume(bestExID)
				reason := zstr.Spaced("BalanceLoad stopJob:", bestBalanceJobID, bestLeft, "exID:", bestExID, "ranAt:", bestRunTime, capacities)
				s.stopJob(bestBalanceJobID, false, false, false, reason)
				continue // we do a new round here instead of refresh
			}
		}
		break
	}
	if s.stopped && len(s.executors) == 0 && len(s.runs) == 0 {
		s.setup.HandleSituationFastFunc(Run[I]{}, SchedulerFinishedStopping, "")
		return
	}
	var nextReason string
	var nextTimerTime time.Time
	var timerJob string
	for _, r := range s.runs {
		if r.Job.Duration == 0 {
			continue
		}
		if !r.RanAt.IsZero() {
			// zlog.Warn("set timer has run")
			jobEnd := r.RanAt.Add(r.Job.Duration)
			if nextTimerTime.IsZero() || nextTimerTime.Sub(jobEnd) > 0 {
				timerJob = r.Job.DebugName
				nextReason = fmt.Sprint("jobEnd:", r.Job.DebugName)
				nextTimerTime = jobEnd
				if s.setup.KeepJobsBeyondAtEndUntilEnoughSlack != 0 && time.Since(nextTimerTime) > 0 {
					nextTimerTime = time.Now().Add(s.setup.KeepJobsBeyondAtEndUntilEnoughSlack)
					nextReason = fmt.Sprint("jobEnd+KeepBeyond:", r.Job.DebugName)
					// d := -time.Since(nextTimerTime)
					// if d > time.Second {
					// 	zlog.Warn("SetNextTimer for slack:", d)
					// }
				}
			}
		}
	}
	if s.setup.ExecutorAliveDuration != 0 {
		for _, e := range s.executors {
			if !e.KeptAliveAt.IsZero() {
				expires := e.KeptAliveAt.Add(s.setup.ExecutorAliveDuration)
				if nextTimerTime.IsZero() || expires.Before(nextTimerTime) {
					nextTimerTime = expires
					nextReason = fmt.Sprint("expires:", e.DebugName)
				}
			}
		}
	}
	s.timer.Stop()
	if nextTimerTime.IsZero() {
		starting := 0
		for _, r := range s.runs {
			if !r.StartedAt.IsZero() {
				starting++
			}
		}
		if !s.stopped && !s.timerOn && (len(s.executors) != 0 && len(s.runs) != 0) && starting == 0 {
			// zlog.Error("No timer, yet we have runs or executors:", len(s.runs), len(s.executors), s.CountJobs(s.zeroID), s.CountRunningJobs(s.zeroID), "starting:", starting, s.setup.ExecutorAliveDuration)
		}
		s.timerOn = false
		return
	}
	d := -time.Since(nextTimerTime)
	// zlog.Warn("SetNextTimer:", d)
	// if d < -time.Second {
	if d <= -20*time.Millisecond {
		limit := zlog.Limit("zsched.NextTime.", timerJob)
		zlog.Warn(limit, "NextTime set to past:", d, "for:", timerJob, nextReason)
	}
	// zlog.Warn("SetTimer:", time.Now().Add(d))
	s.timerOn = true
	s.timer.Reset(d)
}

type capacity struct {
	load               float64
	capacity           float64
	startingCount      int
	mostRecentStarting time.Time //!!!!!!!!!!!!!! use this to not run 2 jobs on same worker after each other!
}

func (c capacity) spare() float64 {
	return c.capacity - c.load
}

func (c capacity) usedRatio() float64 {
	return 1 - c.spare()/c.capacity
}

func (c capacity) unusedRatio() float64 {
	return 1 - c.usedRatio()
}

func (s *Scheduler[I]) startStopViaChannelNonBlocking() {
	select {
	case s.refreshCh <- struct{}{}:
		break
	default:
		break
	}
}

// pushNonBlockingToChannel pushes a value to a channel without blocking/waiting
func pushNonBlockingToChannel[T any](ch chan T, val T) {
	// ch <- val // lets just do it since we made channel
	select {
	case ch <- val:
		break
	default:
		break
	}
}

func (s *Scheduler[I]) calculateLoadOfUsableExecutors() map[I]capacity {
	m := map[I]capacity{}
	runnableEx := s.runnableExecutorIDs()
	// zlog.Warn("calculateLoadOfUsableExecutors", runnableEx)
	for _, e := range s.executors {
		if !runnableEx[e.ID] {
			continue
		}
		m[e.ID] = capacity{capacity: e.CostCapacity}
	}
	for _, r := range s.runs {
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

func (s *Scheduler[I]) startJob(run *Run[I], load map[I]capacity) bool {
	jobID := run.Job.ID
	var bestExID I
	var bestStartingCount = -1
	// var bestCapacity float64
	var bestFull float64
	now := time.Now()
	// zlog.Warn("startJob1:", run.Job.ID, run.ExecutorID, len(s.executors), load, run.Job.Duration, run.Stopping, run.StoppedAt)
	var str string
	for exID, cap := range load {
		// zlog.Warn("startJob On exe?", exID, cap.load)
		e, _ := s.findExecutor(exID)
		if e == nil {
			str += " NoExecutor "
			continue
		}
		// if e == nil {
		// 	zlog.Warn("Exes:", s.executors)
		// }
		// zlog.Warn("startJob1 cap:", exID, e != nil, load)
		exCap := e.CostCapacity - cap.load
		exFull := cap.load / e.CostCapacity
		if exCap < run.Job.Cost {
			str += " ExCapNotEnough "
			continue
		}
		// zlog.Warn("startJob?:", cap.startingCount, run.Job.DebugName, exID, zlog.Full(cap), bestStartingCount, s.setup.SimultaneousStarts, s.setup.MinDurationBetweenSimultaneousStarts)
		if s.setup.SimultaneousStarts != 0 && cap.startingCount >= s.setup.SimultaneousStarts {
			str += zstr.Spaced(" StartingCount ", cap.startingCount, ">=", s.setup.SimultaneousStarts, "in", e.DebugName)
			continue
		}
		if cap.startingCount > 0 && s.setup.SimultaneousStarts != 0 && s.setup.SimultaneousStarts > 1 {
			if time.Since(cap.mostRecentStarting) < s.setup.MinDurationBetweenSimultaneousStarts {
				str += " SimultaniousStarts "
				continue
			}
		}
		str += fmt.Sprint(" • ex:", exID, exFull, cap.load, e.CostCapacity)
		if bestStartingCount == -1 || cap.startingCount < bestStartingCount || exFull < bestFull { // exCap > bestCapacity {
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
			// zlog.Warn("Best:", exID, bestFull, bestStartingCount == -1, cap.startingCount <= bestStartingCount, exFull < bestFull)
			bestFull = exFull
			bestExID = exID
			bestStartingCount = cap.startingCount
		}
	}
	if bestExID == s.zeroID {
		// zlog.Warn("NoWorkersToRunJob:", run.Job.DebugName, str, "loads:", len(load))
		s.setup.HandleSituationFastFunc(*run, NoWorkersToRunJob, str)
		return false
	}
	e, _ := s.findExecutor(bestExID)
	// stop := s.shouldStopJob(*run, e, load)
	// if stop {
	// 	zlog.Warn("startJob:", run.Job.DebugName, bestExID, e.DebugName, str, e.changedCount)
	// }

	// zlog.Warn("startJob:", run.Job.DebugName, bestExID, e.DebugName, str, e.changedCount)
	s.setDebugState(run.Job.ID, false, true, false, false)
	run.StoppedAt = time.Time{}
	run.Removing = false
	run.StartedAt = now
	run.executorChangedCount = e.changedCount
	run.ExecutorID = bestExID
	// run.Starting = true
	d, _ := s.Debug.Get(run.Job.ID)
	d.ExecutorName = e.DebugName
	s.Debug.Set(run.Job.ID, d)
	ctx, cancel := context.WithDeadline(context.Background(), now.Add(s.setup.SlowStartJobFuncTimeout))
	run.Count++
	// zlog.Warn("STARTING JOB:", jobID, run.Count)
	run.starting = true
	s.setup.HandleSituationFastFunc(*run, JobStarted, "")
	runCopy := *run
	go func() {
		err := s.setup.StartJobOnExecutorFunc(runCopy, ctx)
		r, _ := s.findRun(jobID)
		if r == nil {
			reason := zstr.Spaced("Job deleted during execute:", jobID, err)
			s.setup.HandleSituationFastFunc(runCopy, ErrorStartingJob, reason)
			return
		}
		if err != nil {
			r.starting = false
			r.ErrorAt = time.Now()
			reason := zstr.Spaced(jobID, "StartJobOnExecutorFunc done err", err)
			s.setup.HandleSituationFastFunc(runCopy, ErrorStartingJob, reason)
			s.endRunCh <- jobID
			return
		}
		r.ErrorAt = time.Time{}
		r.starting = false
		if s.setup.JobIsRunningOnSuccessfullStart {
			if !r.StoppedAt.IsZero() || r.Stopping || r.ExecutorID == s.zeroID || r.Removing || s.stopped {
				// zlog.Warn("Job Start ended with job/sheduler stopping", jobID)
			} else {
				s.JobIsRunningCh <- jobID
			}
		}
	}()
	cancel()
	go func() {
		s.refreshCh <- struct{}{} // Do this via a goroutine or it gets stuck, even with s.startStopViaChannelNonBlocking()
	}()
	return true
}

func (s *Scheduler[I]) changeExecutor(e Executor[I]) {
	fe, _ := s.findExecutor(e.ID)
	if fe == nil {
		s.addExecutor(e)
		return
	}
	changed := (fe.CostCapacity != e.CostCapacity || fe.SettingsHash != e.SettingsHash)
	// if changed {
	// 	zlog.Warn("changeExecutor", fe.CostCapacity, e.CostCapacity, fe.SettingsHash, e.SettingsHash, (fe.CostCapacity != e.CostCapacity || fe.SettingsHash != e.SettingsHash))
	// }
	fe.CostCapacity = e.CostCapacity
	fe.DebugName = e.DebugName
	fe.Paused = e.Paused
	fe.SettingsHash = e.SettingsHash
	if changed {
		fe.changedCount++
	}
	s.startAndStopRuns() // this will handle Paused changed as well as flushing jobs if changedCount increased
}

func (s *Scheduler[I]) addExecutor(e Executor[I]) {
	// zlog.Warn("addExecutor")
	_, i := s.findExecutor(e.ID)
	if i != -1 {
		zlog.Error("already exists:", e.ID)
		return
	}
	s.executors = append(s.executors, e)
	s.startAndStopRuns()
}

func (s *Scheduler[I]) changeJob(job Job[I]) {
	for i, r := range s.runs {
		if r.Job.ID == job.ID {
			s.runs[i].Job.DebugName = job.DebugName
			s.runs[i].Job.Duration = job.Duration
			s.runs[i].Job.Cost = job.Cost
			if s.setup.ChangingJobRestartsIt {
				reason := zstr.Spaced("changeJob stop:", job.DebugName)
				s.stopJob(job.ID, false, false, true, reason)
			} else {
				s.startAndStopRuns() // a new cost could start it for example
			}
			return
		}
	}
	zlog.Error("zscheduler.changeJob: no such job:", job.DebugName)
}

func (s *Scheduler[I]) purgeStartedJobsNotInList(jobsOnExe JobsOnExecutor[I]) {
	for _, r := range s.runs {
		if r.ExecutorID != jobsOnExe.ExecutorID {
			continue
		}
		var has bool
		for _, j := range jobsOnExe.JobIDs {
			// zlog.Warn("purgeStartedJobsNotInList:", j, r.StartedAt, r.starting)
			if r.Job.ID == j {
				has = true
				break
			}
		}
		if !has {
			// zlog.Warn("Purge?:", r.Job.ID, has, r.StartedAt, r.starting)
			if !r.StartedAt.IsZero() && r.starting {
				// zlog.Warn("Not purging job cause starting", r.Job.ID, time.Since(r.StartedAt))
				continue
			}
			if s.setup.GracePeriodForJobsOnExecutorCh != 0 && !r.StartedAt.IsZero() && time.Since(r.StartedAt) < s.setup.GracePeriodForJobsOnExecutorCh {
				// zlog.Warn("Not purging job cause < grace")
				continue
			}
			reason := zstr.Spaced("purgeJob NotInList:", r.Job.DebugName, r.Job.ID, jobsOnExe)
			// zlog.Warn("PurgeJob:", r.Job.ID)
			s.stopJob(r.Job.ID, false, false, false, reason)
		}
	}
}

func (s *Scheduler[I]) setJobRunning(jobID I) {
	r, _ := s.findRun(jobID)
	if r == nil {
		zlog.Error("JobIsRunningCh on non-existing job", jobID)
		return
	}
	s.setDebugState(jobID, false, false, false, true)
	r.RanAt = time.Now()
	s.setup.HandleSituationFastFunc(*r, JobRunning, "")
	s.startAndStopRuns()
}

func (s *Scheduler[I]) removeExecutor(exID I) {
	// for i := 0; i < len(s.runs); i++ {
	// 	r := s.runs[i]
	// 	if r.ExecutorID == exID && !r.Stopping || !r.Removing {
	// 		s.stopJob(r.Job.ID, true, false)
	// 		i--
	// 	}
	// }
	_, i := s.findExecutor(exID)
	if i == -1 {
		zlog.Error("remove: no executor with id", exID, zlog.CallingStackString())
		return
	}
	zslice.RemoveAt(&s.executors, i)
	s.startAndStopRuns()
}

func (s *Scheduler[I]) removeRun(jobID I) {
	_, i := s.findRun(jobID)
	if i == -1 {
		zlog.Error("removeRun: job not found", jobID)
		return
	}
	// zlog.Warn(r.Job.DebugName, "removeRun", jobID, len(s.runs))
	// was := len(s.runs)
	// name := r.Job.DebugName
	if i != -1 {
		zslice.RemoveAt(&s.runs, i)
	}
	// zlog.Warn(name, "removeRun", jobID, was, len(s.runs))
}

func (s *Scheduler[I]) HasExecutor(exID I) bool {
	e, _ := s.findExecutor(exID)
	return e != nil
}

func (s *Scheduler[I]) CountJobs(executorID I) int {
	var count int
	for _, r := range s.runs {
		if executorID == s.zeroID || r.ExecutorID == executorID {
			count++
		}
	}
	return count
}

func (s *Scheduler[I]) CountStartedJobs(executorID I) int {
	var count int
	for _, r := range s.runs {
		if r.ExecutorID != s.zeroID && (executorID == s.zeroID || r.ExecutorID == executorID) && !r.StartedAt.IsZero() {
			count++
		}
	}
	return count
}

func (s *Scheduler[I]) CountRunningJobs(executorID I) int {
	var count int
	for _, r := range s.runs {
		if (executorID == s.zeroID || r.ExecutorID == executorID) && !r.RanAt.IsZero() {
			count++
		}
	}
	return count
}

// CountRunningJobsWithAMilestone returns the number of running jobs that have reached a milestone, even if it is long past
func (s *Scheduler[I]) CountRunningJobsWithAMilestone(executorID I) int {
	var count int
	for _, r := range s.runs {
		if (executorID == s.zeroID || r.ExecutorID == executorID) && !r.RanAt.IsZero() && !r.MilestoneAt.IsZero() {
			count++
		}
	}
	return count
}

// GetActiveJobIDs gets map of job-ids-to-executor-id of jobs that are running or started for exID or all if it's 0
func (s *Scheduler[I]) GetActiveJobIDs(exID I) map[I]I {
	m := map[I]I{}
	for _, r := range s.runs {
		// zlog.Warn("GetActiveJobIDs:", r.Job.ID, r.ExecutorID, r.StartedAt, r.RanAt)
		if r.ExecutorID != s.zeroID && (exID == s.zeroID || exID == r.ExecutorID) && (!r.StartedAt.IsZero() || !r.RanAt.IsZero()) {
			// zlog.Warn("GetActiveJobIDs Add:", r.Job.ID)
			m[r.Job.ID] = r.ExecutorID
		}
	}
	// zlog.Warn("GetActiveJobIDs2", m)
	return m
}

func (s *Scheduler[I]) GetRun(jobID I) (Run[I], bool) {
	for _, r := range s.runs {
		if r.Job.ID == jobID {
			return r, true
		}
	}
	return Run[I]{}, false
}

func (r *Run[I]) Progress() float64 {
	if r.Job.Duration == 0 {
		return -1
	}
	if !r.RanAt.IsZero() {
		return float64(time.Since(r.RanAt)) / float64(r.Job.Duration)
	}
	if !r.StartedAt.IsZero() {
		return 0
	}
	return -1
}

func (s *Scheduler[I]) endRun(jobID I) {
	defer s.startAndStopRuns()
	r, i := s.findRun(jobID)
	if i == -1 {
		if !s.stopped {
			zlog.Error("endRun: job not found", jobID, len(s.runs))
		}
		return
	}
	// zlog.Warn("endRun:", jobID, r.Stopping, r.Removing, len(s.runs), r.Removing, s.stopped, r.ExecutorID)
	rc := *r
	if r.Removing || s.stopped {
		// zlog.Warn("removing")
		s.removeRun(jobID)
	} else {
		r.starting = false
		r.StartedAt = time.Time{}
		r.RanAt = time.Time{}
		r.StoppedAt = time.Time{}
		r.MilestoneAt = time.Time{}
		r.Removing = false
		r.Stopping = false
		r.ExecutorID = s.zeroID // do this after so HandleSituationFastFunc has it
	}
	// zlog.Warn("endRun2:", jobID, len(s.runs))
	s.setup.HandleSituationFastFunc(rc, JobEnded, "")
}

func (s *Scheduler[I]) isExecutorAlive(e *Executor[I]) bool {
	if e == nil {
		return true // it is alive if it doesn't exist, how else will it become alive?
	}
	if s.setup.ExecutorAliveDuration == 0 {
		return true
	}
	// zlog.Warn("isExecutorAlive?", time.Since(e.KeptAliveAt), s.ExecutorAliveDuration)
	return time.Since(e.KeptAliveAt) <= s.setup.ExecutorAliveDuration
}

func (s *Scheduler[I]) Runs() []Run[I] {
	runs := make([]Run[I], len(s.runs))
	copy(runs, s.runs)
	return runs
}

func (s *Scheduler[I]) Executors() []Executor[I] {
	ex := make([]Executor[I], len(s.executors))
	copy(ex, s.executors)
	return ex
}

func (s *Scheduler[I]) Stop() {
	s.stopped = true
	s.executors = []Executor[I]{}
	for i := 0; i < len(s.runs); i++ {
		r := s.runs[i]
		if !r.RanAt.IsZero() || r.Stopping {
			continue
		}
		zslice.RemoveAt(&s.runs, i)
		i--
	}
	s.refreshCh <- struct{}{}
}

func (s *Scheduler[I]) CopyOfSetup() Setup[I] {
	return s.setup
}

func (s *Scheduler[I]) GetRunForID(jobID I) (Run[I], error) {
	// for _, r := range s.runs {
	// 	zlog.Warn("GetRunForIDs:", r.Job.ID, r.Count)
	// }
	r, _ := s.findRun(jobID)
	if r == nil {
		return Run[I]{}, zlog.Error("no job:", jobID)
	}
	// zlog.Warn("Run4job:", r.Job.ID, r.Count)
	return *r, nil
}
