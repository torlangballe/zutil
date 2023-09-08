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

var startTotal, endTotal time.Duration

func TestAdd(t *testing.T) {
	b := NewScheduler[int64]()
	b.SimultaneousStarts = 2
	b.MinDurationBetweenSimultaneousStarts = time.Millisecond * 300
	b.ExecutorAliveDuration = 0
	b.LoadBalanceIfCostDifference = 2
	b.KeepJobsBeyondAtEndUntilEnoughSlack = time.Second * 20
	b.StartJobOnExecutorSlowFunc = func(run Run[int64], ctx context.Context) error {
		//		zlog.Warn(jobID, "start:") //, executorID)
		d := time.Millisecond * 200
		if run.ExecutorID == 8 {
			d *= 5
		}
		time.Sleep(d)
		startTotal += d
		//		zlog.Warn(jobID, "start done:") //, executorID)
		// b.JobIsRunningCh <- run.Job.ID
		return nil
	}
	b.StopJobOnExecutorSlowFunc = func(run Run[int64], ctx context.Context) error {
		d := time.Millisecond * 20
		time.Sleep(d)
		endTotal += d
		return nil
	}
	b.HandleSituationFastFunc = func(run Run[int64], s SituationType, err error) {
		if s == JobStarted || s == JobRunning || s == JobStopped || s == JobEnded {
			b.DebugPrintExecutors(run, s)
		}
		// zlog.Warn("situation:", s, err)
	}
	go b.Start()
	worker := Executor[int64]{
		ID:           1,
		CostCapacity: 30,
		KeptAliveAt:  time.Now(),
		DebugName:    "Wrk1",
	}
	b.AddExecutorCh <- worker

	worker8 := Executor[int64]{
		ID:           8,
		CostCapacity: 30,
		KeptAliveAt:  time.Now(),
		DebugName:    "Wrk8",
	}
	b.AddExecutorCh <- worker8

	for i := 0; i < 30; i++ {
		addJob(b, int64(i+1))
		//		addJobRandomly(b, job)
		//removeJobRandomly(b, job.ID)
	}
	// ztimer.StartIn(9, func() {
	// 	w := worker8
	// 	w.Paused = true
	// 	b.ChangeExecutorCh <- w
	// })
	// ztimer.StartIn(15, func() {
	// 	b.ChangeExecutorCh <- worker8 // with !paused
	// })
	// ztimer.RepeatForever(5, func() {
	// 	var st, et time.Duration
	// 	b.Debug.ForEach(func(key int64, row JobDebug) bool {
	// 		st += row.Started
	// 		et += row.Ended
	// 		debugRow(row)
	// 		return true
	// 	})
	// 	fmt.Println("tstarted:", startTotal, "sum:", st)
	// 	fmt.Println("tended:", endTotal, "sum:", et)
	// 	fmt.Println()
	// })
	select {}
}

func randDurSecs(min, max float64) float64 {
	return min + (max-min)*rand.Float64()
}

func removeJobRandomly(b *Scheduler[int64], jobID int64) {
	ztimer.StartIn(randDurSecs(1, 2), func() {
		go func() {
			b.RemoveJobCh <- jobID
		}()
		removeJobRandomly(b, jobID)
	})
}

func addJobRandomly(b *Scheduler[int64], job Job[int64]) {
	ztimer.StartIn(randDurSecs(1, 2), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			b.AddJobCh <- job
		}()
		addJobRandomly(b, job)
	})
}

func addJob(b *Scheduler[int64], id int64) Job[int64] {
	job := Job[int64]{
		ID:        id,
		DebugName: fmt.Sprint("J", id),
		Duration:  time.Second*10 + time.Millisecond*200*time.Duration(id),
		Cost:      1,
	}
	b.AddJobCh <- job
	return job
}

func addTime(d time.Duration, t time.Time) time.Duration {
	if !t.IsZero() {
		d += time.Since(t)
	}
	return d
}

func debugRow(row JobDebug) {
	kn := time.Since(row.Known)
	ex := addTime(row.Existed, row.Existing)
	st := addTime(row.Started, row.Starting)
	en := addTime(row.Ended, row.Ending)
	ru := addTime(row.Runned, row.Running)
	zlog.Warn(row.JobName, row.ExecutorName, "known:", kn, "existed:", ex, "starting:", st, "ending:", en, "run:", ru, "gone:", kn-ex-st-en-ru)
}
