package zprocess

import (
	"testing"
	"time"

	"github.com/torlangballe/zutil/ztimer"
)

func TestAdd(t *testing.T) {
	b := NewBalancer[int64]()
	b.SimultaneousStarts = 2
	b.MinDurationBetweenSimultaneousStarts = time.Millisecond * 300
	b.StartJobOnExecutorFunc = func(jobID, executorID int64) error {
		//		zlog.Warn(jobID, "start:") //, executorID)
		time.Sleep(time.Millisecond * 50)
		//		zlog.Warn(jobID, "start done:") //, executorID)
		b.JobIsRunningCh <- jobID
		return nil
	}
	b.StopJobOnExecutor = func(jobID, executorID int64) error {
		//zlog.Warn(jobID, "stop:") //, executorID)
		return nil
	}
	b.HandleSituationFunc = func(jobID, executorID int64, s SituationType, err error) {
		// zlog.Warn("situation:", s, err)
	}
	go b.Start()
	worker := Executor[int64]{
		ID:          1,
		On:          true,
		Paused:      false,
		Spend:       10,
		KeptAliveAt: time.Now(),
	}
	b.AddExecutorCh <- worker

	job2 := Job[int64]{
		ID:        2,
		DebugName: "Job2",
		Duration:  time.Second * 2,
		On:        true,
		Cost:      1,
	}
	b.AddJobCh <- job2

	job3 := Job[int64]{
		ID:        3,
		DebugName: "Job3",
		Duration:  time.Second*3 + time.Millisecond*200,
		On:        true,
		Cost:      2,
	}
	b.AddJobCh <- job3

	ztimer.StartIn(20, func() {
		b.TouchExecutorCh <- 1
	})
	select {}
}
