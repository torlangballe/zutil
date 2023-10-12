package zscheduler

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

const (
	startMS = 20
	stopMS  = 10
)

func makeScheduler(jobs, workers int, jobCost, workerCap float64) *Scheduler[int64] {
	s := NewScheduler[int64]()
	setup := DefaultSetup[int64]()
	setup.SimultaneousStarts = 1
	setup.ExecutorAliveDuration = 0
	setup.LoadBalanceIfCostDifference = 2
	setup.JobIsRunningOnSuccessfullStart = true
	setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 20
	setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * startMS)
		return nil
	}
	setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {
		if sit == JobStarted || sit == JobRunning || sit == JobStopped || sit == JobEnded {
			// s.DebugPrintExecutors(run, sit)
		}
	}
	setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * stopMS)
		return nil
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
	fmt.Println("TestChangeExecutor")
	s := makeScheduler(20, 2, 1, 10)
	time.Sleep(time.Second * 2)
	e := makeExecutor(s, 2, 11)
	s.ChangeExecutorCh <- e
	time.Sleep(time.Millisecond * 40)
	c2 := s.CountJobs(2)
	if c2 == 10 {
		t.Error("No reduced jobs shortly after changing executor")
	}
	time.Sleep(time.Second)
	c2 = s.CountJobs(2)
	if c2 != 10 {
		t.Error("Jobs not back to 10 a while after changing executor", c2)
	}
	stopAndCheckScheduler(s, t)
}

func testLoadBalance1(t *testing.T) {
	fmt.Println("TestLoadBalance1")
	s := makeScheduler(20, 1, 1, 10)
	// s.setup.LoadBalanceIfCostDifference = 2
	time.Sleep(time.Second)
	c1 := s.CountJobs(1)
	compare(t, "Jobs not 10 on w1", c1, 10)
	s.AddExecutorCh <- makeExecutor(s, int64(2), 10)
	time.Sleep(time.Second * 2)
	c1 = s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 10/10", c1, 10, c2, 10)
	stopAndCheckScheduler(s, t)
}

func testLoadBalance2(t *testing.T) {
	fmt.Println("TestLoadBalance2")
	s := makeScheduler(20, 1, 1, 20)
	// s.setup.LoadBalanceIfCostDifference = 2

	time.Sleep(time.Second)
	c1 := s.CountJobs(1)
	compare(t, "Jobs not 20 on executor1", c1, 20)
	s.AddExecutorCh <- makeExecutor(s, int64(2), 10)
	// fmt.Println("** TestLoadBalance AddExecutor2")
	time.Sleep(time.Second * 2)
	c1 = s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 14/6", c1, 14, c2, 6)
	stopAndCheckScheduler(s, t)
}

func testStartingTime(t *testing.T) {
	fmt.Println("TestStartingTime")
	s := makeScheduler(10, 1, 1, 20)
	s.setup.LoadBalanceIfCostDifference = 0
	c1 := s.CountRunningJobs(1)
	compare(t, "Jobs not starting at 0:", c1, 0)
	time.Sleep(time.Millisecond * 20 * 9)
	c1 = s.CountRunningJobs(1)
	if c1 == 0 || c1 == 10 {
		t.Error("Jobs not still 0 or reached 10:", c1, 10, c1, 0)
	}
	time.Sleep(time.Millisecond * startMS * 3)
	c1 = s.CountRunningJobs(1)
	compare(t, "Jobs not reached 10:", c1, 10)
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
		zlog.Error(nil, zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
	}
	return !fail
}

func testPauseWithCapacity(t *testing.T) {
	fmt.Println("TestPauseWithCapacity")
	s := makeScheduler(20, 2, 1, 10)
	time.Sleep(time.Second * 2)
	c1 := s.CountJobs(1)
	c2 := s.CountJobs(2)
	compare(t, "Jobs not spread 10/10:", c1, 10, c2, 10)
	e1, _ := s.findExecutor(1)
	e1.Paused = true
	s.ChangeExecutorCh <- *e1
	time.Sleep(time.Second)
	c1 = s.CountJobs(1)
	c2 = s.CountJobs(2)
	compare(t, "Jobs not spread 0/10 after pause without capacity beyond 10:", c1, 0, c2, 10)
	e2, _ := s.findExecutor(2)
	e2.CostCapacity = 20
	s.ChangeExecutorCh <- *e2
	time.Sleep(time.Second * 2)
	c1 = s.CountJobs(1)
	c2 = s.CountJobs(2)
	compare(t, "Jobs not spread 0/20 after pause now with capacity at 20:", c1, 0, c2, 20)
	stopAndCheckScheduler(s, t)
}

func stopAndCheckScheduler(s *Scheduler[int64], t *testing.T) {
	sleep := (startMS + stopMS) * (len(s.runs) + 1)
	// zlog.Warn("*** stopAndCheckScheduler", sleep)
	s.Stop()
	time.Sleep(time.Duration(sleep) * time.Millisecond)
	if !compare(t, "length of runs should be zero", len(s.runs), 0) {
		zlog.Warn("should be zero:", s.runs)
	}
	compare(t, "length of executors should be zero", len(s.executors), 0)
	if s.timerOn {
		t.Error("timers still on a while after stop")
	}
}

func testStartStop(t *testing.T) {
	var timers []*ztimer.Timer
	zlog.Warn("TestStartStop")
	s := makeScheduler(0, 0, 0, 0)
	for i := 0; i < 30; i++ {
		t := addAndRemoveJobRandomly(s, makeJob(s, int64(i+1), time.Second, 1))
		timers = append(timers, t)
	}
	for i := 0; i < 10; i++ {
		t := addAndRemoveExecutorRandomly(s, makeExecutor(s, int64(i+1), 5))
		timers = append(timers, t)
	}
	time.Sleep(time.Second * 5)
	// zlog.Warn("*** TestStartStop All Stopped:", s.CountJobs(0))
	for _, t := range timers {
		t.Stop()
	}
	s.Stop()
	stopAndCheckScheduler(s, t)
}

func testKeepAlive(t *testing.T) {
	fmt.Println("TestKeepAlive")
	s := makeScheduler(10, 1, 1, 10)
	s.setup.ExecutorAliveDuration = time.Second
	s.SetExecutorIsAliveCh <- 1
	time.Sleep(time.Millisecond * 900)
	count := s.CountJobs(1)
	compare(t, "Jobs still at 10 before not alive:", count, 10)
	time.Sleep(time.Millisecond * 400)
	count = s.CountJobs(1)
	if count == 10 {
		t.Error("Jobs still at 10 after not alive:", count)
	}
}

func testOverMax(t *testing.T) {
	fmt.Println("TestOverMax")
	s := makeScheduler(10, 1, 1, 10)
	s.setup.TotalMaxJobCount = 5
	time.Sleep(time.Millisecond * 900)
	count := s.CountJobs(1)
	compare(t, "Jobs not still at 5 which is max:", count, 5)
	s.SetTotalMaxJobCountCh <- 10
	time.Sleep(time.Millisecond * 500)
	count = s.CountJobs(1)
	compare(t, "Jobs now at 10 which is now max:", count, 10)
}

func testSetJobsOnExecutor(t *testing.T) {
	const jobCount = 8
	fmt.Println("TestSetJobsOnExecutor")
	s := makeScheduler(0, 1, 1, 10)
	time.Sleep(time.Second)
	for i := 0; i < 7; i++ {
		time.Sleep(time.Second)
		var je JobsOnExecutor[int64]
		// for j := 0; j < jobCount/2; j++ {
		// 	je.JobIDs = append(je.JobIDs, rand.Int63n(jobCount)+1)
		// 	je.ExecutorID = 1
		// }
		s.SetJobsOnExecutorCh <- je
	}
}

func testEnoughRunning(t *testing.T) {
	const jobCount = 6
	fmt.Println("TestEnoughRunning")
	s := makeScheduler(0, 1, 1, 10)
	s.setup.KeepJobsBeyondAtEndUntilEnoughSlack = time.Millisecond * 800
	s.setup.SimultaneousStarts = 1
	s.setup.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		if rand.Int31n(4) == 2 {
			time.Sleep(time.Millisecond * 300)
			return errors.New("random error")
		}
		return nil
	}
	s.setup.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		return nil
	}
	for i := 0; i < jobCount; i++ {
		job := makeJob(s, int64(i+1), time.Second, 1)
		s.AddJobCh <- job
	}
	time.Sleep(time.Second * (jobCount + 1))
	// zlog.Warn("Count:", s.CountRunningJobs(0))
	s.setup.HandleSituationFastFunc = func(run Run[int64], sit SituationType, details string) {
		count := s.CountRunningJobs(0)
		if count < jobCount-1 {
			t.Error("Not enough jobs running:", count)
		}
		// zlog.Warn("TestEnoughRunning Count", count)
	}
	time.Sleep(time.Second * 7)
}

func makeJob(b *Scheduler[int64], id int64, dur time.Duration, cost float64) Job[int64] {
	job := Job[int64]{
		ID:        id,
		DebugName: fmt.Sprint("J", id),
		Duration:  dur,
		Cost:      cost,
	}
	return job
}

func makeExecutor(b *Scheduler[int64], id int64, cap float64) Executor[int64] {
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

func TestAll(t *testing.T) {
	testSetJobsOnExecutor(t)
	// testEnoughRunning(t)
	// testChangeExecutor(t)
	// testStartStop(t)
	// testPauseWithCapacity(t)
	// testStartingTime(t)
	// testLoadBalance1(t)
	// testLoadBalance2(t)
	// testKeepAlive(t)
	// testOverMax(t)
}
