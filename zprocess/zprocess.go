package zprocess

import (
	"context"
	"math/rand"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// OnceWait is something you call Wait() on and it waits until Done() is called on it. Once.
// Use to wait for some global data to be inited for example.
// TODO: Check if a semaphore could be used.
type OnceWait struct {
	done   bool
	inited bool
	wg     sync.WaitGroup
}

var (
	MainThreadExeCh chan func()
	procs           zmap.LockMap[int64, *proc]
	lastProcPrint   time.Time
)

func init() {
	zdebug.GetOngoingProcsCountFunc = func() int {
		return procs.Count()
	}
}

// PoolWorkOnItems runs jobs with do(), processing all in goroutines,
// But up to max poolSize at a time.
func PoolWorkOnItems[T any](all []T, poolSize int, do func(t *T)) {
	length := len(all)
	jobs := make(chan *T, length)
	results := make(chan struct{}, length)
	for i := 0; i < poolSize; i++ {
		go func() {
			for j := range jobs {
				do(j)
				results <- struct{}{}
			}
		}()
	}
	for i := range all {
		jobs <- &all[i]
	}
	close(jobs)
	for range all {
		<-results
	}
}

// RunFuncUntilTimeoutSecs uses RunFuncUntilContextDone to wait secs for a function to finish,
// or returns while it's still running in a goroutine.
func RunFuncUntilTimeoutSecs(secs float64, do func()) (completed bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ztime.SecondsDur(secs))
	completed = RunFuncUntilContextDone(ctx, do)
	cancel()
	return completed
}

// RunFuncUntilContextDone waits for do() to finish or the context to be done
// If it finishes it returns completed = true, otherwise the goroutine continues, but it returns with false.
func RunFuncUntilContextDone(ctx context.Context, do func()) (completed bool) {
	_, has := ctx.Deadline()
	if !has {
		do()
		return true
	}
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
	p.stack = zdebug.CallingStackStringAt(2)
	p.info = info
	procs.Set(p.id, &p)
	p.timer = ztimer.StartIn(timeoutSecs, func() {
		zlog.Error("Process timed out:\n", info, p.stack)
	})
	return &p
}

func PopProcess(p *proc) {
	p.timer.Stop()
	procs.Remove(p.id)
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

// OnThreadExecutor returns a channel of func() to push functions you want to run on the thread OnThreadExecutor was called on.
// the rest function is called in a go-routine

func OnThreadExecutor(c *chan func(), rest func()) chan func() {
	runtime.LockOSThread()
	*c = make(chan func())
	if rest != nil {
		go rest()
	}
	for {
		select {
		case f := <-*c:
			zlog.Info("Got Func")
			f()
		}
	}
}

func StartMainThreadExecutor(rest func()) {
	OnThreadExecutor(&MainThreadExeCh, rest)
}

func RunFuncInMainThread(f func()) {
	MainThreadExeCh <- f
}

func RunAndWaitForFuncInMainThread(f func()) {
	var wg sync.WaitGroup

	wg.Add(1)
	zlog.Info("RunAndWaitForFuncInMainThread", MainThreadExeCh != nil)
	MainThreadExeCh <- func() {
		zlog.Info("here1")
		f()
		wg.Done()
	}
	zlog.Info("here")
	wg.Wait()
}

// QuoteCommandLineArgument makes a string possible to use as an argument in a unix shell
// Not that this is just for other uses than os/exec which escapes arguments.
func QuoteCommandLineArgument(s string) string {
	if len(s) == 0 {
		return "''"
	}
	regEx := regexp.MustCompile(`[^\w@%+=:,./-]`)
	if regEx.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}
	return s
}
