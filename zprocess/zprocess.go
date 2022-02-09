package zprocess

import (
	"context"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"

	"github.com/torlangballe/zutil/ztime"
)

func RunFuncUntilTimeoutSecs(secs float64, do func()) (completed bool) {
	ctx, _ := context.WithTimeout(context.Background(), ztime.SecondsDur(secs))
	return RunFuncUntilContextDone(ctx, do)
}

func RunFuncUntilContextDone(ctx context.Context, do func()) (completed bool) {
	doneChannel := make(chan struct{}, 2)
	go func() {
		do()
		doneChannel <- struct{}{}
	}()
	select {
	case <-doneChannel:
		return true
	case <-ctx.Done():
		return false
	}
}

func WaitTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		defer close(c)
		wg.Wait()
	}()
	select {
	case <-c:
		return false // completed normally
	case <-time.After(timeout):
		return true // timed out
	}
}

type TimedMutex struct {
	mutex    sync.Mutex
	repeater *ztimer.Repeater
}

func (t *TimedMutex) Lock() {
	stack := zlog.GetCallingStackString()
	timer := ztimer.StartIn(5, func() {
		zlog.Info("🟥TimeMutex slow lock > 5 sec:", stack)
	})
	start := time.Now()
	t.mutex.Lock()
	timer.Stop()
	t.repeater = ztimer.RepeatIn(5, func() bool {
		zlog.Info("🟥TimeMutex still locked for:", time.Since(start), stack)
		return true
	})
}

func (t *TimedMutex) Unlock() {
	t.repeater.Stop()
	t.mutex.Unlock()
}
