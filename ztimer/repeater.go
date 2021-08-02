package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type Repeater struct {
	ticker *time.Ticker
	stop   chan bool
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

func (r *Repeater) Set(secs float64, now bool, perform func() bool) {
	// zlog.Info("\nTimer Repeater:", secs, now, zlog.GetCallingStackString(), "\n")
	r.Stop()
	r.ticker = time.NewTicker(ztime.SecondsDur(secs))
	r.stop = make(chan bool, 1)
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
		for {
			select {
			case <-ch:
				if !perform() {
					r.stop <- true
				}
			case <-r.stop:
				return
			}
		}
	}()
}

func (r *Repeater) Stop() {
	if r.ticker != nil {
		r.ticker.Stop()
		r.stop <- true
		r.ticker = nil
	}
}

func (r *Repeater) IsStopped() bool {
	return (r.ticker == nil)
}
