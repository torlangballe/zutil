package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"fmt"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
)

var (
	GoingCount int

	repeaters      = map[string]int{}
	repeatersMutex sync.Mutex
)

type Repeater struct {
	ticker *time.Ticker
	stop   chan bool
	stack  string
}

func RepeaterNew() *Repeater {
	return &Repeater{}
}

func Repeat(secs float64, perform func() bool) *Repeater {
	r := RepeaterNew()
	r.Set(secs, false, perform)
	return r
}

func RepeatForever(secs float64, perform func()) *Repeater {
	r := RepeaterNew()
	r.Set(secs, false, func() bool {
		perform()
		return true
	})
	return r
}

func RepeatForeverNow(secs float64, perform func()) *Repeater {
	r := RepeaterNew()
	r.Set(secs, true, func() bool {
		perform()
		return true
	})
	return r
}

func RepeatNow(secs float64, perform func() bool) *Repeater {
	r := RepeaterNew()
	r.Set(secs, true, perform)
	return r
}

var stopped int

func (r *Repeater) Set(secs float64, now bool, perform func() bool) {
	if r.ticker != nil {
		r.ticker.Stop()
	}
	invokeFunc := zdebug.FileLineAndCallingFunctionString(4, true)
	r.ticker = time.NewTicker(secs2Dur(secs))
	repeatersMutex.Lock()
	r.stack = zdebug.FileLineAndCallingFunctionString(4, true)
	// zlog.Info("Repeater.Set():", r.stack)
	repeaters[r.stack]++
	repeatersMutex.Unlock()
	GoingCount++
	r.stop = make(chan bool, 5)
	go func() {
		defer zdebug.RecoverFromPanic(true, invokeFunc)
		if now {
			if !perform() {
				return
			}
		}
		t := r.ticker
		if t == nil {
			return
		}
		ch := t.C //
		doing := false
		for {
			select {
			case <-ch:
				if doing {
					continue
				}
				doing = true
				if !perform() {
					doing = false
					r.stop <- true
					// don't return here, must do case <-stop: below
				}
				doing = false
			case <-r.stop:
				stopped++
				GoingCount--
				repeatersMutex.Lock()
				// zlog.Info("Repeater <-r.stop:", r.stack, repeaters[r.stack])
				repeaters[r.stack]--
				repeatersMutex.Unlock()
				if r.ticker != nil {
					r.ticker.Stop()
					r.ticker = nil
				}
				return
			}
		}
	}()
}

func (r *Repeater) Stop() {
	if r != nil && r.ticker != nil {
		r.stop <- true
	}
}

func (r *Repeater) IsStopped() bool {
	return (r.ticker == nil)
}

func DumpRepeaters() {
	repeatersMutex.Lock()
	zlog.Info("Repeaters:", "going:", GoingCount, "stopped:", stopped, "unique:", len(repeaters))
	for s, n := range repeaters {
		if n > 15 {
			fmt.Println("RepeaterCount (>5):", n, ":", s)
		}
	}
	repeatersMutex.Unlock()
}

func RepeatAtMostEvery(secs float64, do func() bool) {
	go func() {
		for {
			start := time.Now()
			if !do() {
				return
			}
			left := secs - secsSince(start)
			if left > 0 {
				time.Sleep(secs2Dur(left))
			}
		}
	}()
}

func secs2Dur(secs float64) time.Duration {
	return time.Duration(secs * float64(time.Second))
}

func dur2Secs(d time.Duration) float64 {
	return float64(d) / float64(time.Second)
}
