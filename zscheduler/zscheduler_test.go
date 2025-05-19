package zscheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

const startMSDefault = 2

var (
	startMS = startMSDefault
	stopMS  = 1
)

var stopping bool

func startDuration() time.Duration {
	return time.Millisecond * time.Duration(startMS)
}

func newScheduler(jobs, workers int, jobCost, workerCap float64, setupFunc func(*Setup[int64])) *Scheduler[int64] {
	s := NewScheduler[int64]()
	setup := DefaultSetup[int64]()
	setup.SimultaneousStarts = 1
	setup.ExecutorAliveDuration = 0
	setup.LoadBalanceIfCostDifference = 2
	setup.JobIsRunningOnSuccessfulStart = true
	setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 2
	setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		// zlog.Warn("StartingJobSlow:", run.Job.ID, run.Count, time.Millisecond*time.Duration(startMS))
		time.Sleep(time.Millisecond * time.Duration(startMS))
		// zlog.Warn("StartingJobSlow End:", run.Job.ID)
		return nil
	}
	setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {
		if sit == JobStarted || sit == JobRunning || sit == JobStopped || sit == JobEnded {
			if sit == JobStopped {
				// zlog.Warn("STOPJOB:", run.Job.DebugName, run.ExecutorID)
			}
			// zlog.Warn("Sit:", sit)
			// s.DebugPrintExecutors(run, sit)
		}
	}
	setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * time.Duration(stopMS))
		return nil
	}
	if setupFunc != nil {
		setupFunc(&setup)
	}
	s.Init(setup)
	go s.Start()
	for i := 0; i < jobs; i++ {
		job := makeJob(s, int64(i+1), time.Second*30, jobCost)
		s.AddJobCh <- job
	}
	for i := 0; i < workers; i++ {
		s.AddExecutorCh <- makeExecutor(s, int64(i+1), workerCap)
	}
	return s
}

func testChangeExecutor(t *testing.T) {
	fmt.Println("testChangeExecutor")
	s := newScheduler(20, 2, 1, 10, nil)
	time.Sleep(time.Millisecond * 200)
	e := makeExecutor(s, 2, 11)
	s.ChangeExecutorCh <- e
	time.Sleep(time.Millisecond * 4)
	c2 := s.CountJobs(2)
	if c2 == 10 {
		t.Error("No reduced jobs shortly after changing executor")
	}
	time.Sleep(time.Millisecond * 300)
	c2 = s.CountJobs(2)
	if c2 != 10 {
		t.Error("Jobs not back to 10 a while after changing executor", c2)
	}
	// zlog.Warn("ReadyToStop")
	stopAndCheckScheduler(s, t)
}

func testLoadBalance1(t *testing.T) {
	fmt.Println("testLoadBalance1")
	s := newScheduler(20, 1, 1, 10, nil)
	// s.setup.LoadBalanceIfCostDifference = 2
	time.Sleep(time.Second)
	c1 := s.CountJobs(1)
	compare(t, "Jobs not 10 on w1", c1, 10)
	s.AddExecutorCh <- makeExecutor(s, int64(2), 10)
	time.Sleep(time.Millisecond * 200)
	c1 = s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 10/10", c1, 10, c2, 10)
	stopAndCheckScheduler(s, t)
}

func testLoadBalance2(t *testing.T) {
	fmt.Println("testLoadBalance2")
	s := newScheduler(20, 1, 1, 20, nil)
	// s.setup.LoadBalanceIfCostDifference = 2

	time.Sleep(time.Millisecond * 100)
	c1 := s.CountJobs(1)
	compare(t, "Jobs not 20 on executor1", c1, 20)
	s.AddExecutorCh <- makeExecutor(s, int64(2), 10)
	// fmt.Println("** TestLoadBalance AddExecutor2")
	time.Sleep(time.Millisecond * 200)
	c1 = s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 14/6", c1, 14, c2, 6)
	stopAndCheckScheduler(s, t)
}

func testStartingTime(t *testing.T) {
	fmt.Println("testStartingTime")
	s := newScheduler(10, 1, 1, 20, nil)
	s.setup.LoadBalanceIfCostDifference = 0
	c1 := s.CountRunningJobs(1)
	compare(t, "Jobs not starting at 0:", c1, 0)
	time.Sleep(time.Millisecond * time.Duration(startMS) * 8) // wait for 8 of them to start
	c1 = s.CountRunningJobs(1)
	if c1 == 0 || c1 == 10 {
		t.Error("Jobs still 0 or reached 10:", c1, 10, c1, 0)
	}
	time.Sleep(startDuration() * 10) // wait past when all 10 should be started, give a bit extra to make sure
	c1 = s.CountRunningJobs(1)
	compare(t, "Jobs not reached 10:", c1, 10)
	stopAndCheckScheduler(s, t)
}

func testPauseWithCapacity(t *testing.T) {
	fmt.Println("testPauseWithCapacity")
	s := newScheduler(20, 2, 1, 10, nil)
	time.Sleep(time.Millisecond * 200)
	c1 := s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 10/10:", c1, 10, c2, 10)
	e1, _ := s.findExecutor(1)
	// zlog.Warn("SetPause on e1")
	e1.Paused = true
	s.ChangeExecutorCh <- *e1
	time.Sleep(time.Millisecond * 200)
	c1 = s.CountJobs(1)
	c2 = s.CountJobs(2)
	compare(t, "Jobs not spread 0/10 after pause without capacity beyond 10:", c1, 0, c2, 10)
	e2, _ := s.findExecutor(2)
	e2.CostCapacity = 20
	s.ChangeExecutorCh <- *e2
	// zlog.Warn("Here")
	time.Sleep(time.Millisecond * 200)
	c1 = s.CountJobs(1)
	c2 = s.CountJobs(2)
	compare(t, "Jobs not spread 0/20 after pause now with capacity at 20:", c1, 0, c2, 20)
	// zlog.Warn("Here2")
	stopAndCheckScheduler(s, t)
}

func testPauseWithTwoExecutors(t *testing.T) {
	fmt.Println("testPauseWithTwoExecutors")
	s := newScheduler(20, 2, 1, 20, nil)
	time.Sleep(time.Millisecond * 200)
	c1 := s.CountRunningJobs(1)
	c2 := s.CountRunningJobs(2)
	if c1 < 9 || c1 > 11 {
		t.Error("Executor1 has wrong amount of jobs:", c1)
	}
	if c2 < 9 || c2 > 11 {
		t.Error("Executor2 has wrong amount of jobs:", c1)
	}
	e2 := makeExecutor(s, 2, 20)
	e2.Paused = true
	s.ChangeExecutorCh <- e2
	time.Sleep(time.Millisecond * 10)
	c1 = s.CountRunningJobs(1)
	c2 = s.CountRunningJobs(2)
	// zlog.Warn("After pause and sleep", c1, c2)
	if c2 == 0 {
		t.Error("Executor2 drained too fast after pause:", c2, c1)
	}
	time.Sleep(time.Second * 3)
	c1 = s.CountRunningJobs(1)
	c2 = s.CountRunningJobs(2)
	if c1 != 20 || c2 != 0 {
		t.Error("Executor not at 20/0 after slow drain:", c1, c2)
	}
	// zlog.Warn("Before exit")
	stopAndCheckScheduler(s, t)
}

func testStartStop(t *testing.T) {
	var timers []*ztimer.Timer
	fmt.Println("testStartStop")
	s := newScheduler(0, 0, 0, 0, nil)
	for i := 0; i < 30; i++ {
		t := addAndRemoveJobRandomly(s, makeJob(s, int64(i+1), time.Second, 1))
		timers = append(timers, t)
	}
	for i := 0; i < 10; i++ {
		t := addAndRemoveExecutorRandomly(s, makeExecutor(s, int64(i+1), 5))
		timers = append(timers, t)
	}
	time.Sleep(time.Millisecond * 500)
	// zlog.Warn("*** TestStartStop All Stopped:", s.CountJobs(0))
	for _, t := range timers {
		t.Stop()
	}
	s.Stop()
	stopAndCheckScheduler(s, t)
}

func testKeepAlive(t *testing.T) {
	fmt.Println("testKeepAlive")
	s := newScheduler(10, 1, 1, 10, nil)
	s.setup.ExecutorAliveDuration = time.Millisecond * 100
	s.SetExecutorIsAliveCh <- 1
	time.Sleep(time.Millisecond * 90)
	count := s.CountJobs(1)
	compare(t, "Jobs still at 10 before not alive:", count, 10)
	// zlog.Warn("Before sleep beyond alive")
	time.Sleep(time.Millisecond * 20)
	count = s.CountRunningJobs(1)
	if count == 10 {
		t.Error("Jobs still at 10 after not alive:", count)
	}
	stopAndCheckScheduler(s, t)
	s.setup.ExecutorAliveDuration = 0
}

func testStopAndCheck(t *testing.T) {
	fmt.Println("testStopAndCheck")
	s := newScheduler(10, 1, 1, 10, nil)
	time.Sleep(time.Millisecond * 100)
	count := s.CountJobs(1)
	compare(t, "Jobs at 10 after sleep:", count, 10)
	stopAndCheckScheduler(s, t)
}

func testOverMax(t *testing.T) {
	fmt.Println("testOverMax")
	s := newScheduler(10, 1, 1, 10, nil)
	s.setup.TotalMaxJobCount = 5
	time.Sleep(time.Millisecond * 90)
	count := s.CountJobs(1)
	compare(t, "Jobs not still at 5 which is max:", count, 5)
	s.SetTotalMaxJobCountCh <- 10
	time.Sleep(time.Millisecond * 50)
	count = s.CountJobs(1)
	compare(t, "Jobs now at 10 which is now max:", count, 10)
	stopAndCheckScheduler(s, t)
}

func testSetJobsOnExecutor(t *testing.T) {
	const jobCount = 8
	fmt.Println("testSetJobsOnExecutor")
	s := newScheduler(0, 1, 1, 10, nil)
	time.Sleep(time.Millisecond * 100)
	for i := 0; i < 7; i++ {
		time.Sleep(time.Millisecond * 100)
		var je JobsOnExecutor[int64]
		// for j := 0; j < jobCount/2; j++ {
		// 	je.JobIDs = append(je.JobIDs, rand.Int63n(jobCount)+1)
		// 	je.ExecutorID = 1
		// }
		s.SetJobsOnExecutorCh <- je
	}
	stopAndCheckScheduler(s, t)
}

func testEnoughRunning(t *testing.T) {
	fmt.Println("testEnoughRunning")
	const jobCount = 6
	s := newScheduler(0, 2, 1, 10, nil) // 2: We need at least to executors to ensure only one job ever not running
	s.setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Millisecond * 5000
	s.setup.SimultaneousStarts = 1
	s.setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		if rand.Int31n(4) == 2 {
			time.Sleep(time.Millisecond * 30)
			return errors.New("random error")
		}
		return nil
	}
	s.setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {
		// s.DebugPrintExecutors(run, sit)
	}
	s.setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		return nil
	}
	for i := 0; i < jobCount; i++ {
		job := makeJob(s, int64(i+1), time.Second, 1)
		s.AddJobCh <- job
	}
	time.Sleep(time.Millisecond * 100 * (jobCount + 1))
	// zlog.Warn("Count:", s.CountRunningJobs(0))
	s.setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {
		// s.DebugPrintExecutors(run, sit)
		if s.stopped {
			return
		}
		count := s.CountRunningJobs(0)
		if count < jobCount-1 {
			zlog.Error("Not enough jobs running:", sit, "count:", count, "<", jobCount-1, s.CountJobs(0))
			s.stopped = true
			t.Error("Not enough jobs running")
			// t.Fatal(time.Now(), "Not enough jobs running:", sit, "count:", count, "<", jobCount-1, s.CountJobs(0))
		}
		// zlog.Warn("TestEnoughRunning Count", count)
	}
	time.Sleep(time.Millisecond * 700)
	s.setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {}
	stopAndCheckScheduler(s, t)
}

func testPurgeFromRunningList(t *testing.T) {
	fmt.Println("testPurgeFromRunningList")
	s := newScheduler(4, 1, 1, 10, nil)
	time.Sleep(time.Millisecond * 100)
	// count := s.CountJobs(0)
	// compare(t, "Jobs not at 4:", count, 4)
	// r2, _ := s.GetRunForID(2)
	// compare(t, "RunCount not one for job2:", r2.Count, 1)
	// var j JobsOnExecutor[int64]
	// j.JobIDs = []int64{1, 3, 4}
	// j.ExecutorID = 1
	// s.SetJobsOnExecutorCh <- j
	// time.Sleep(time.Second)
	// r2, _ = s.GetRunForID(2)
	// compare(t, "RunCount not now 2 for job2:", r2.Count, 2)
	stopAndCheckScheduler(s, t)
}

func testPurgeFromSlowStarting(t *testing.T) {
	fmt.Println("testPurgeFromSlowStarting")
	startMS = 10
	s := newScheduler(4, 1, 1, 10, func(setup *Setup[int64]) {
		setup.SimultaneousStarts = 0
	})
	time.Sleep(startDuration() / 3) // enough time to have jobs registered, but not be running
	count := s.CountJobs(0)
	compare(t, "Jobs not at 4:", count, 4) // All 4 jobs running
	countRunning := s.CountRunningJobs(0)
	compare(t, "Running Jobs not still 0:", countRunning, 0)
	r2, err := s.GetRunForID(2)
	if err != nil {
		t.Error(err)
	}
	compare(t, "RunCount not 1 for job j:", r2.Count, 1) // RunCount of job1 is 1
	var j JobsOnExecutor[int64]
	j.JobIDs = []int64{1, 3, 4}
	j.ExecutorID = 1
	// time.Sleep(time.Millisecond * 10)
	s.SetJobsOnExecutorCh <- j // We say only jobs 1, 3, and 4 are running on exector1, while job 2 is still starting
	time.Sleep(time.Millisecond * 100)
	r2, _ = s.GetRunForID(2)
	compare(t, "RunCount not still 1 for job 2:", r2.Count, 1) // yet since job 2 is still starting, we don't nuke it, so runcount is still 1

	time.Sleep(time.Millisecond * 1010)

	//	time.Sleep(time.Millisecond * 100) // make sure sleep(1)+sleep(1) is more than job 2's start time
	s.SetJobsOnExecutorCh <- j         // We again say only jobs 1, 3, and 4 are running on exector1, once job 2 has finished starting
	time.Sleep(time.Millisecond * 200) //100                        // Wait a bit for job 2 to be stopped and started

	r2, _ = s.GetRunForID(2)
	startMS = startMSDefault
	compare(t, "RunCount not finaly 2 for job2:", r2.Count, 2) // run count for job 2 should now be 2
	count = s.CountStartedJobs(0)
	compare(t, "Jobs not at 4 again:", count, 4) // All 4 jobs running again
	startMS = 10
	time.Sleep(time.Millisecond * 10)

	s.AddJobCh <- makeJob(s, 5, time.Second*10, 1)
	time.Sleep(time.Millisecond * 50)
	count = s.CountStartedJobs(0)
	compare(t, "Jobs count not 5 after add:", count, 5)
	r5, _ := s.GetRunForID(5)
	compare(t, "Job 5 Run-Count not 1:", r5.Count, 1)
	j.JobIDs = []int64{1, 2, 3, 4} // We don't set job 5
	s.setup.GracePeriodForJobsOnExecutorCh = time.Second * 5
	s.SetJobsOnExecutorCh <- j
	time.Sleep(time.Millisecond * 50) // 100 msec total
	r5, _ = s.GetRunForID(5)
	compare(t, "Job 5 Run-Count not still 1 since purge doesn't happen while starting:", r5.Count, 1)
	time.Sleep(time.Millisecond * 200) // 300 ms total
	s.SetJobsOnExecutorCh <- j
	time.Sleep(time.Millisecond * 10)
	r5, _ = s.GetRunForID(5)
	compare(t, "Job 5 Run-Count not still 1 since purge doesn't happen while grace not up:", r5.Count, 1)
	startMS = startMSDefault

	// time.Sleep(time.Millisecond * 200) // 5+ total
	// s.SetJobsOnExecutorCh <- j
	// time.Sleep(time.Millisecond * 12)
	// r5, _ = s.GetRunForID(5)
	// compare(t, "Job 5 Run-Count not 2 after purge finally happens:", r5.Count, 2)

	stopAndCheckScheduler(s, t)
}

func setupTestPlayWithSlackAfterDurationEnd() *Scheduler[int64] {
	s := newScheduler(0, 1, 1, 10, func(setup *Setup[int64]) {
		setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
			if run.Job.ID == 2 {
				time.Sleep(time.Millisecond * 60)
				return nil
			}
			time.Sleep(time.Millisecond)
			return nil
		}
	})
	job := makeJob(s, int64(1), time.Second, 1)
	s.AddJobCh <- job
	time.Sleep(time.Millisecond * 1)
	jobSlow := makeJob(s, 2, time.Second, 1)
	s.AddJobCh <- jobSlow
	return s
}

func testPlayWithSlackLongWait(t *testing.T) {
	fmt.Println("testPlayWithSlackLongWait")
	s := setupTestPlayWithSlackAfterDurationEnd()
	s.setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 20
	time.Sleep(time.Millisecond * 10)
	r1, _ := s.GetRunForID(1)
	r2, _ := s.GetRunForID(2)
	compare(t, "Job1 Run count not 1:", r1.Count, 1)
	compare(t, "Job2 Run count not 1:", r2.Count, 1) // even though not running yet, still has count==1
	time.Sleep(time.Millisecond * 200)
	r1, _ = s.GetRunForID(1)
	compare(t, "Job1 Run count not 1:", r1.Count, 1) // should still be 1, since too too slow to finish playing
	// zlog.Warn("Done**************************\n\n")
	stopAndCheckScheduler(s, t)
}

func testPlayWithSlackShortWait(t *testing.T) {
	fmt.Println("testPlayWithSlackShortWait")
	s := setupTestPlayWithSlackAfterDurationEnd()
	s.setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 1
	time.Sleep(time.Millisecond * time.Duration(startMS))
	r1, _ := s.GetRunForID(1)
	compare(t, "Job1 Run count not 1:", r1.Count, 1)
	time.Sleep(time.Millisecond * 65)
	count := s.CountRunningJobs(0)
	compare(t, "Running jobs not 2:", count, 2) // should be 0, as 1 is blocked by 2 starting, and 2 hasn't started yet
	// zlog.Warn("Done**************************\n\n")
	stopAndCheckScheduler(s, t)
}

func setupTestPlayForMilestone() *Scheduler[int64] {
	s := newScheduler(0, 1, 1, 10, func(setup *Setup[int64]) {
		setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Millisecond * 200
		setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
			time.Sleep(time.Millisecond * 1)
			return nil
		}
	})
	job := makeJob(s, int64(1), time.Millisecond*100, 1)
	s.AddJobCh <- job
	return s
}

func testPlayWithSlackLongWaitAndMilestone(t *testing.T) {
	fmt.Println("testPlayWithSlackLongWaitAndMilestone")
	timings := []float64{
		0.1,  // normal 1 second until stop
		0.25, // we setup StopJobIfSinceMilestoneLessThan before and set SetJobHasMilestoneNowCh at 2.5, so it happens here.
		0.35, // We turn off StopJobIfSinceMilestoneLessThan, so next stop is a second later
		0.65, // We turn it on again, but Milestone is so old, it stops when KeepJobsBeyondAtEndUntilEnoughSlack kicks in
	}
	start := time.Now()
	s := setupTestPlayForMilestone()
	i := 0
	ztimer.StartIn(0.12, func() {
		// zlog.Warn("SetMilestoneDur")
		s.setup.StopJobIfSinceMilestoneLessThan = time.Second
		s.refreshCh <- struct{}{}
	})
	ztimer.StartIn(0.25, func() {
		s.SetJobHasMilestoneNowCh <- 1 // we set it just had a milestone (thumb in qtt), so should
	})
	ztimer.StartIn(0.27, func() {
		s.setup.StopJobIfSinceMilestoneLessThan = 0
	})
	ztimer.StartIn(0.37, func() {
		s.setup.StopJobIfSinceMilestoneLessThan = time.Second
	})
	var done bool
	s.setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		if i >= len(timings) {
			done = true
			return nil
		}
		since := ztime.Since(start)
		diff := math.Abs(since - timings[i])
		if diff > 0.1 {
			t.Error("timing #", i, since, "not near enough to:", timings[i])
		}
		// zlog.Warn("Stopped:", since)
		i++
		return nil
	}
	for !done {
		time.Sleep(time.Millisecond * 20)
	}
	stopAndCheckScheduler(s, t)
}

func testErrorAt(t *testing.T) {
	fmt.Println("testErrorAt")
	s := newScheduler(0, 1, 1, 10, func(setup *Setup[int64]) {
		setup.SimultaneousStarts = 0
		setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
			return nil
		}
		setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
			return nil
		}
	})
	for i := 0; i < 2; i++ {
		job := makeJob(s, int64(i+1), time.Second, 1)
		s.AddJobCh <- job
	}
	time.Sleep(time.Millisecond * 20)
	first := true
	s.setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		if first && run.Job.ID != 1 {
			t.Error("Job1 should start first, not:", run.Job.ID)
		}
		first = false
		return nil
	}
	time.Sleep(time.Millisecond * 100)

	// second run has happened
	s.SetJobHasErrorCh <- 1
	time.Sleep(time.Millisecond * 20)
	e := makeExecutor(s, 1, 0)
	s.ChangeExecutorCh <- e
	time.Sleep(time.Millisecond * 10)
	e = makeExecutor(s, 1, 10)
	s.ChangeExecutorCh <- e
	first = true
	s.setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		// zlog.Warn("third startups:", run.Job.DebugName, first, run.ErrorAt)
		if first && run.Job.ID != 2 {
			t.Error("Job2 should start first, since 1 got error, not:", run.Job.ID)
		}

		first = false
		return nil
	}
	time.Sleep(time.Millisecond * 170)
	stopAndCheckScheduler(s, t)
}

func compare(t *testing.T, str string, n ...int) bool {
	var fail bool
	for i := 0; i < len(n); i += 2 {
		c := n[i]
		val := n[i+1]
		if c != val {
			str += fmt.Sprint(" ", c, "!=", val)
			fail = true
		}
	}
	if fail {
		zlog.Error(zlog.StackAdjust(2), "Fail:", str)
		t.Error(str)
	}
	return !fail
}

func makeJob(s *Scheduler[int64], id int64, dur time.Duration, cost float64) Job[int64] {
	job := Job[int64]{
		ID:        id,
		DebugName: fmt.Sprint("J", id),
		Duration:  dur,
		Cost:      cost,
	}
	return job
}

func makeExecutor(s *Scheduler[int64], id int64, cap float64) Executor[int64] {
	e := Executor[int64]{
		ID:           id,
		CostCapacity: cap,
		KeptAliveAt:  time.Now(),
		DebugName:    fmt.Sprint("Wrk", id),
	}
	return e
}

func randDurSecs(min, max float64) float64 {
	return min + (max-min)*rand.Float64()
}

func addAndRemoveJobRandomly(s *Scheduler[int64], job Job[int64]) *ztimer.Timer {
	timer := ztimer.TimerNew()
	timer.StartIn(randDurSecs(1, 2), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		if s.stopped {
			return
		}
		go func() {
			s.AddJobCh <- job
		}()
		timer.StartIn(randDurSecs(3, 2), func() {
			if s.stopped {
				return
			}
			s.RemoveJobCh <- job.ID
			addAndRemoveJobRandomly(s, job)
		})
	})
	return timer
}

func addAndRemoveExecutorRandomly(s *Scheduler[int64], e Executor[int64]) *ztimer.Timer {
	timer := ztimer.TimerNew()
	timer.StartIn(randDurSecs(2, 3), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		if s.stopped {
			return
		}
		go func() {
			s.AddExecutorCh <- e
		}()
		ztimer.StartIn(randDurSecs(8, 12), func() {
			if s.stopped {
				return
			}
			s.RemoveExecutorCh <- e.ID
			addAndRemoveExecutorRandomly(s, e)
		})
	})
	return timer
}

func stopAndCheckScheduler(s *Scheduler[int64], t *testing.T) {
	// zlog.Warn("stopAndCheckScheduler")
	sleep := (startMS + stopMS) * (len(s.runs) + 1)
	s.Stop()
	time.Sleep(time.Duration(sleep) * time.Millisecond)
	if !compare(t, "stopAndCheckScheduler: length of runs should be zero", len(s.runs), 0) {
		zlog.Warn("should be zero:", sleep)
	}
	compare(t, "stopAndCheckScheduler: length of executors should be zero", len(s.executors), 0)
	if s.timerOn {
		t.Error("timers still on a while after stop")
	}
}

func testMixedAttributes(t *testing.T) {
	var AMask = []string{"cdn.1"}
	var BMask = []string{"cdn.2"}
	fmt.Println("testMixedAttributes")
	s := newScheduler(0, 0, 1, 20, nil)
	e := makeExecutor(s, 1, 20)
	e.AcceptAttributes = AMask
	s.ChangeExecutorCh <- e

	job := makeJob(s, 1, time.Second*1, 1)
	job.Attributes = BMask
	s.AddJobCh <- job

	time.Sleep(time.Millisecond * 200)
	count := s.CountRunningJobs(1)
	if count > 0 {
		t.Error("Executor shouldn't have any jobs yet:", count)
	}
	time.Sleep(time.Millisecond * 200)
	job = makeJob(s, 2, time.Second*1, 1)
	job.Attributes = []string{} //
	// zlog.Warn("ADD:", job.Attributes)
	s.AddJobCh <- job
	time.Sleep(time.Millisecond * 200)
	count = s.CountRunningJobs(1)
	if count != 1 {
		t.Error("Executor should have one job now:", count)
	}

	time.Sleep(time.Millisecond * 200)
	job = makeJob(s, 3, time.Second*1, 1)
	job.Attributes = AMask
	s.AddJobCh <- job
	time.Sleep(time.Millisecond * 200)
	count = s.CountRunningJobs(1)
	if count != 2 {
		t.Error("Executor should have two jobs now:", count)
	}

	e2 := makeExecutor(s, 2, 20)
	e2.AcceptAttributes = BMask
	s.ChangeExecutorCh <- e2
	// zlog.Warn("ADD Exe2", e2.DebugName)

	time.Sleep(time.Millisecond * 200)
	count = s.CountRunningJobs(0)
	if count != 3 {
		t.Error("Executor should have have 3 jobs:", count)
	}

	stopAndCheckScheduler(s, t)
}

func TestAll(t *testing.T) {
	// testEnoughRunning(t)
	// testPauseWithTwoExecutors(t)
	// testSetJobsOnExecutor(t)
	// testChangeExecutor(t)
	// testStartStop(t)
	// testPauseWithCapacity(t)
	// testStartingTime(t)
	// testLoadBalance1(t)
	// testLoadBalance2(t)
	// testKeepAlive(t)
	// testStopAndCheck(t)
	// testOverMax(t)
	// testPurgeFromRunningList(t)
	// testPurgeFromSlowStarting(t)
	// testPlayWithSlackLongWait(t)
	// testPlayWithSlackShortWait(t)
	// testPlayWithSlackLongWaitAndMilestone(t)
	// testErrorAt(t)
	testMixedAttributes(t)
}
