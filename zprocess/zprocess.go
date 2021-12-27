package zprocess

import (
	"context"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"

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
	mutex sync.Mutex
	Start time.Time
}

func (t *TimedMutex) Lock() {
	t.Start = time.Now()
	t.mutex.Lock()
	since := time.Since(t.Start)
	// zlog.Info("**TimeMutex lock:", since)
	if since > time.Second*1 {
		zlog.Info("🟥TimeMutex slow lock:", since, zlog.GetCallingStackString())
	}
}

func (t *TimedMutex) Unlock() {
	t.mutex.Unlock()
	since := time.Since(t.Start)
	if since > time.Second*1 {
		zlog.Info("🟥TimeMutex slow unlock:", since, zlog.GetCallingStackString())
	}
}
