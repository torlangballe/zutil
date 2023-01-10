package zprocess

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
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
	stack := zlog.CallingStackString()
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

var (
	procs         zmap.LockMap[int64, *proc]
	lastProcPrint time.Time
)

func RepeatPrintInOutRequests() {
	ztimer.RepeatForever(0.1, func() {
		if time.Since(lastProcPrint) > time.Second*1 {
			procs.ForEach(func(k int64, p *proc) bool {
				if time.Since(p.start) > time.Second*10 {
					zlog.Info("Slow Request:", time.Since(p.start), p.info, p.stack)
					lastProcPrint = time.Now()
					return true
				}
				return true
			})
			count := procs.Count()
			if count > 5 {
				lastProcPrint = time.Now()
				zlog.Info("Ongoing I/O Requests:", count)
			}
		}
	})
}

type proc struct {
	start time.Time
	id    int64
	stack string
	info  string
	timer *ztimer.Timer
}

func PushProcess(timeoutSecs float64, info string) *proc {
	var p proc
	p.start = time.Now()
	p.id = rand.Int63()
	p.stack = zlog.CallingStackStringAt(3)
	p.info = info
	procs.Set(p.id, &p)
	p.timer = ztimer.StartIn(timeoutSecs, func() {
		zlog.Error(nil, "Process timed out:\n", info, p.stack)
	})
	return &p
}

func PopProcess(p *proc) {
	p.timer.Stop()
	procs.Delete(p.id)
}
