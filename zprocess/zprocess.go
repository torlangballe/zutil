package zprocess

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
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
	sync.Mutex
}

/*
var currentMutexes zmap.LockMap[int64, *TimedMutex]

type TimedMutex struct {
	mutex    sync.Mutex
	start    time.Time
	started  time.Time
	stack    string
	id       int64
	repeater *ztimer.Repeater
}

const warnSecs = 10

func reportExisting(skipID int64) {
	currentMutexes.ForEach(func(id int64, t *TimedMutex) bool {
		if id == skipID {
			return true
		}
		zlog.Info("existing lock", time.Since(t.start), time.Since(t.started), t.stack)
		return true
	})
	zlog.Info("")
}

func (t *TimedMutex) Lock() {
	if t.id == 0 {
		t.id = rand.Int63()
	}
	stack := zlog.CallingStackStringAt(1)
	now := time.Now()
	currentMutexes.Set(t.id, t)

	t.start = now
	timer := ztimer.StartIn(warnSecs, func() {
		zlog.Info("🟥TimeMutex slow to lock:", time.Since(now), "secs:", stack, "original lock:", t.stack)
		reportExisting(t.id)
	})
	t.stack = stack
	t.mutex.Lock()
	t.started = time.Now()
	currentMutexes.Remove(t.id)
	timer.Stop()
	t.repeater = ztimer.Repeat(warnSecs, func() bool {
		zlog.Info("🟥TimeMutex still locked for:", time.Since(t.start), stack)
		reportExisting(t.id)
		return true
	})
}

func (t *TimedMutex) Unlock() {
	t.repeater.Stop()
	t.mutex.Unlock()
	since := ztime.Since(t.started)
	if since > warnSecs {
		zlog.Info("🟥TimeMutex was locked for", since, "seconds", t.stack)
	}
}
*/

var (
	procs         zmap.LockMap[int64, *proc]
	lastProcPrint time.Time
)

func RepeatPrintInOutRequests() {
	ztimer.RepeatForever(0.1, func() {
		if time.Since(lastProcPrint) > time.Second*1 {
			procs.ForEach(func(k int64, p *proc) bool {
				if time.Since(p.start) > time.Second*10 {
					zlog.Info("Slow Request:", time.Since(p.start), "count:", p.info, p.stack)
					lastProcPrint = time.Now()
					return true
				}
				return true
			})
			count := procs.Count()
			if count > 10 {
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
	p.stack = zlog.CallingStackStringAt(2)
	p.info = info
	procs.Set(p.id, &p)
	p.timer = ztimer.StartIn(timeoutSecs, func() {
		zlog.Error(nil, "Process timed out:\n", info, p.stack)
	})
	return &p
}

func PopProcess(p *proc) {
	p.timer.Stop()
	procs.Remove(p.id)
}

type OnceWait struct {
	done   bool
	inited bool
	wg     sync.WaitGroup
}

func (o *OnceWait) Wait() {
	if !o.inited {
		o.inited = true
		o.wg.Add(1)
	}
	o.wg.Wait()
}

func (o *OnceWait) Done() {
	if !o.inited {
		o.inited = true
		o.wg.Add(1)
	}
	if !o.done {
		o.done = true
		o.wg.Done()
	}
}
