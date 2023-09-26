package zprocess

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

func makeScheduler(jobs, workers int, jobCost, workerCap float64) *Scheduler[int64] {
	b := NewScheduler[int64]()
	b.SimultaneousStarts = 1
	b.ExecutorAliveDuration = 0
	b.LoadBalanceIfCostDifference = 2
	b.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 20
	b.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * 20)
		return nil
	}
	b.HandleSituationFastFunc = func(run Run[int64], s SituationType, err error) {
		if s == JobStarted || s == JobRunning || s == JobStopped || s == JobEnded {
			// b.DebugPrintExecutors(run, s)
		}
	}
	b.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * 10)
		return nil
	}
	go b.Start()
	for i := 0; i < jobs; i++ {
		job := makeJob(b, int64(i+1), time.Second*30, jobCost)
		b.AddJobCh <- job
	}
	for i := 0; i < workers; i++ {
		b.addExector(makeExecutor(b, int64(i+1), workerCap))
	}
	return b
}

func TestLoadBalance1(t *testing.T) {
	fmt.Println("TestLoadBalance1")
	b := makeScheduler(20, 1, 1, 10)
	b.LoadBalanceIfCostDifference = 2
	time.Sleep(time.Second)
	c1 := b.CountJobs(1)
	compare(t, "Jobs not 10 on w1", c1, 10)
	b.addExector(makeExecutor(b, int64(2), 10))
	time.Sleep(time.Second * 2)
	c1 = b.CountJobs(1)
	c2 := b.CountJobs(2)
	compare(t, "Jobs not spread 10/10", c1, 10, c2, 10)
}

func TestLoadBalance2(t *testing.T) {
	fmt.Println("TestLoadBalance2")
	b := makeScheduler(20, 1, 1, 20)
	b.LoadBalanceIfCostDifference = 2
	time.Sleep(time.Second)
	c1 := b.CountJobs(1)
	compare(t, "Jobs not 20 on w1 part 2", c1, 20)
	b.addExector(makeExecutor(b, int64(2), 10))
	time.Sleep(time.Second * 2)
	c1 = b.CountJobs(1)
	c2 := b.CountJobs(2)
	compare(t, "Jobs not spread 14/6", c1, 14, c2, 6)
}

func TestStartingTime(t *testing.T) {
	fmt.Println("TestStartingTime")
	b := makeScheduler(10, 1, 1, 20)
	b.LoadBalanceIfCostDifference = 0
	c1 := b.CountRunningJobs(1)
	compare(t, "Jobs not starting at 0:", c1, 0)
	time.Sleep(time.Millisecond * 20 * 9)
	c1 = b.CountRunningJobs(1)
	if c1 == 0 || c1 == 10 {
		t.Error("Jobs not still 0 or reached 10:", c1, 10, c1, 0)
	}
	time.Sleep(time.Millisecond * 20 * 2)
	c1 = b.CountRunningJobs(1)
	compare(t, "Jobs not reached 10:", c1, 10)
}

func compare(t *testing.T, str string, n ...int) {
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
		t.Error(str)
	}
}

func TestPauseWithCapacity(t *testing.T) {
	fmt.Println("TestPauseWithCapacity")
	b := makeScheduler(20, 2, 1, 10)
	time.Sleep(time.Second * 2)
	c1 := b.CountJobs(1)
	c2 := b.CountJobs(2)
	compare(t, "Jobs not spread 10/10:", c1, 10, c2, 10)
	e1, _ := b.FindExecutor(1)
	e1.Paused = true
	b.ChangeExecutorCh <- *e1
	time.Sleep(time.Second)
	c1 = b.CountJobs(1)
	c2 = b.CountJobs(2)
	compare(t, "Jobs not spread 0/10 after pause without capacity beyond 10:", c1, 0, c2, 10)
	e2, _ := b.FindExecutor(2)
	e2.CostCapacity = 20
	b.ChangeExecutorCh <- *e2
	time.Sleep(time.Second * 2)
	c1 = b.CountJobs(1)
	c2 = b.CountJobs(2)
	compare(t, "Jobs not spread 0/20 after pause now with capacity at 20:", c1, 0, c2, 20)
}

func TestStartStop(t *testing.T) {
	fmt.Println("TestStartStop")
	b := makeScheduler(0, 0, 0, 0)
	for i := 0; i < 30; i++ {
		addAndRemoveJobRandomly(b, makeJob(b, int64(i+1), time.Second, 1))
	}
	for i := 0; i < 10; i++ {
		addAndRemoveExecutorRandomly(b, makeExecutor(b, int64(i+1), 5))
	}
	time.Sleep(time.Second * 10)
	return
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

func addAndRemoveJobRandomly(b *Scheduler[int64], job Job[int64]) *ztimer.Timer {
	timer := ztimer.TimerNew()
	timer.StartIn(randDurSecs(1, 2), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			b.AddJobCh <- job
		}()
		timer.StartIn(randDurSecs(3, 2), func() {
			b.RemoveJobCh <- job.ID
			addAndRemoveJobRandomly(b, job)
		})
	})
	return timer
}

func addAndRemoveExecutorRandomly(b *Scheduler[int64], e Executor[int64]) {
	ztimer.StartIn(randDurSecs(2, 3), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			b.AddExecutorCh <- e
		}()
		ztimer.StartIn(randDurSecs(8, 12), func() {
			b.RemoveExecutorCh <- e.ID
			addAndRemoveExecutorRandomly(b, e)
		})
	})
}

func pauseExecutorRandomly(b *Scheduler[int64], e Executor[int64]) {
	ztimer.StartIn(randDurSecs(10, 7), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			zlog.Warn("Pause", e.DebugName)
			e.Paused = true
			b.ChangeExecutorCh <- e
		}()
		ztimer.StartIn(randDurSecs(12, 3), func() {
			// zlog.Warn("UnPause", e.DebugName)
			// e.Paused = false
			// b.ChangeExecutorCh <- e
		})
	})
}
