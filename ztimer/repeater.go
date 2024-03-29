package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
)

var (
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

// var count int
// var stopped int
// var going int
// var all int

func (r *Repeater) Set(secs float64, now bool, perform func() bool) {
	// zlog.Info("Repeat:", secs, zlog.GetCallingStackString())
	if r.ticker != nil {
		r.Stop()
	}
	r.ticker = time.NewTicker(secs2Dur(secs))
	// repeatersMutex.Lock()
	// r.stack = zlog.FileLineAndCallingFunctionString(4)
	// repeaters[r.stack]++
	// repeatersMutex.Unlock()
	// count++
	// going++
	// all++
	r.stop = make(chan bool, 5)
	go func() {
		defer zlog.HandlePanic(true)
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
				// stopped++
				// repeatersMutex.Lock()
				// repeaters[r.stack]--
				// repeatersMutex.Unlock()
				// count--
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
	// repeatersMutex.Lock()
	// zlog.Info("All Repeaters: Count:", count, "all:", all, "going:", going, "stopped:", stopped, "len:", len(repeaters))
	// for s, n := range repeaters {
	// 	zlog.Info("Repeaters", n, ":", s)
	// }
	// repeatersMutex.Unlock()
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
