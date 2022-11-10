package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
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

func RepeatIn(secs float64, perform func() bool) *Repeater {
	r := RepeaterNew()
	r.Set(secs, false, perform)
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
	r.ticker = time.NewTicker(ztime.SecondsDur(secs))
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
	if r.ticker != nil {
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
