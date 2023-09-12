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
	b.StartJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
		//		zlog.Warn(jobID, "start:") //, executorID)
		d := time.Millisecond * 200
		if run.ExecutorID == int64(5) {
			d *= 5
		}
		time.Sleep(d)
		startTotal += d
		//		zlog.Warn(jobID, "start done:") //, executorID)
		// b.JobIsRunningCh <- run.Job.ID
		return nil
	}
	b.StopJobOnExecutorFunc = func(run Run[int64], ctx context.Context) error {
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

	for i := 0; i < 30; i++ {
		addJob(b, int64(i+1))
		// addJobRandomly(b, job)
	}
	for i := 0; i < 8; i++ {
		e := makeExecutor(b, int64(i+1))
		b.addExector(e)
	}
	// for i := 0; i < 8; i++ {
	// 	e := makeExecutor(b, int64(i+1))
	// 	if i < 4 {
	// 		pauseExecutorRandomly(b, e)
	// 	}
	// }
	// ztimer.StartIn(9, func() {
	// 	w := worker8
	// 	w.Paused = true
	// 	b.ChangeExecutorCh <- w
	// })
	// ztimer.StartIn(15, func() {
	// 	b.ChangeExecutorCh <- worker8 // with !paused
	// })

	ztimer.RepeatForever(5, func() {
		var st, et time.Duration
		b.Debug.ForEach(func(key int64, row JobDebug) bool {
			st += row.Started
			et += row.Ended
			debugRow(row)
			return true
		})
		fmt.Println("tstarted:", startTotal, "sum:", st)
		fmt.Println("tended:", endTotal, "sum:", et)
		fmt.Println()
	})
	select {}
}

func addJob(b *Scheduler[int64], id int64) Job[int64] {
	job := makeJob(b, id)
	b.AddJobCh <- job
	return job
}

func makeJob(b *Scheduler[int64], id int64) Job[int64] {
	job := Job[int64]{
		ID:        id,
		DebugName: fmt.Sprint("J", id),
		Duration:  time.Second*30 + time.Millisecond*200*time.Duration(id),
		Cost:      1,
	}
	return job
}

func makeExecutor(b *Scheduler[int64], id int64) Executor[int64] {
	e := Executor[int64]{
		ID:           id,
		CostCapacity: 10,
		KeptAliveAt:  time.Now(),
		DebugName:    fmt.Sprint("Wrk", id),
	}
	zlog.Warn("mkExe:", id)
	return e
}

func randDurSecs(min, max float64) float64 {
	return min + (max-min)*rand.Float64()
}

func addJobRandomly(b *Scheduler[int64], job Job[int64]) {
	ztimer.StartIn(randDurSecs(1, 2), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			b.AddJobCh <- job
		}()
		ztimer.StartIn(randDurSecs(3, 2), func() {
			b.RemoveJobCh <- job.ID
			addJobRandomly(b, job)
		})
	})
}

func addExecutorRandomly(b *Scheduler[int64], e Executor[int64]) {
	ztimer.StartIn(randDurSecs(2, 3), func() {
		// zlog.Warn("addJobRandomly", job.DebugName)
		go func() {
			b.AddExecutorCh <- e
		}()
		ztimer.StartIn(randDurSecs(8, 12), func() {
			b.RemoveExecutorCh <- e.ID
			addExecutorRandomly(b, e)
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

func TestSimultaneousStarts(t *testing.T) {
	for i :=0; i < 10; i++ {
		
	}
}
