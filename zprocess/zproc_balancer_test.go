package zprocess

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

func TestAdd(t *testing.T) {
	b := NewBalancer[int64]()
	b.SimultaneousStarts = 2
	b.MinDurationBetweenSimultaneousStarts = time.Millisecond * 300
	b.ExecutorAliveDuration = 0
	b.StartJobOnExecutorSlowFunc = func(run Run[int64], ctx context.Context) error {
		//		zlog.Warn(jobID, "start:") //, executorID)
		time.Sleep(time.Millisecond * 50)
		//		zlog.Warn(jobID, "start done:") //, executorID)
		b.JobIsRunningCh <- run.Job.ID
		return nil
	}
	b.StopJobOnExecutorSlowFunc = func(run Run[int64], ctx context.Context) error {
		time.Sleep(time.Millisecond * 70)
		//zlog.Warn(jobID, "stop:") //, executorID)
		return nil
	}
	b.HandleSituationFastFunc = func(run Run[int64], s SituationType, err error) {
		// zlog.Warn("situation:", s, err)
	}
	go b.Start()
	worker := Executor[int64]{
		ID:          1,
		Spend:       10,
		KeptAliveAt: time.Now(),
		DebugName:   "Wrk1",
	}
	b.AddExecutorCh <- worker

	worker2 := Executor[int64]{
		ID:          8,
		Spend:       16,
		KeptAliveAt: time.Now(),
		DebugName:   "Wrk8",
	}
	b.AddExecutorCh <- worker2

	job2 := Job[int64]{
		ID:        2,
		DebugName: "Job2",
		Duration:  time.Second * 2,
		Cost:      1,
	}
	b.AddJobCh <- job2

	job3 := Job[int64]{
		ID:        3,
		DebugName: "Job3",
		Duration:  time.Second*3 + time.Millisecond*200,
		Cost:      2,
	}
	b.AddJobCh <- job3

	ztimer.RepeatForever(5, func() {
		b.Debug.ForEach(func(key int64, row JobDebug) bool {
			debugRow(row)
			return true
		})
		fmt.Println()
	})
	ztimer.StartIn(8, func() {
		zlog.Warn("RemoveJob2")
		b.RemoveJobCh <- 2
	})
	ztimer.StartIn(11, func() {
		zlog.Warn("AddJob2")
		b.AddJobCh <- job2
	})
	select {}
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
