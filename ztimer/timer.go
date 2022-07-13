package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type Timer struct {
	timer *time.Timer
	mutex sync.Mutex
}

// type Stopper interface {
// 	Stop()
// }

func TimerNew() *Timer {
	return &Timer{}
}

func StartIn(secs float64, perform func()) *Timer {
	t := TimerNew()
	t.StartIn(secs, perform)
	return t
}

/*
func (t *Timer) check(perform func()) {
	t.mutex.Lock()
	// fmt.Printf("timer start1: %p %v\n", t, t.timer)
	// fmt.Printf("timer start2: %p %v\n", t, t.timer)
	// fmt.Printf("timer start3: %p %v\n", t, t.timer)
	if t.timer != nil {
		c := t.timer.C
		t.mutex.Unlock()
		// fmt.Printf("timer start 2.5: %p %p\n", t, t.timer)
		<-c
		perform()
	} else {
		t.mutex.Unlock()
		// zlog.Error(nil, "timer start 2.5-II: %p %p\n", t, t.timer)
		//		zlog.Error(nil, "timer was nil")
	}
}

func (t *Timer) StartIn(secs float64, perform func()) *Timer {
	// zlog.Info("timer start1:")
	t.Stop()
	t.timer = time.NewTimer(ztime.SecondsDur(secs))
	go func() {
		defer zlog.HandlePanic(true)
		t.check(perform)
	}()
	return t
}

func (t *Timer) Stop() {
	// fmt.Printf("timer stop: %p %p\n", t, t.timer)
	t.mutex.Lock()
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
	t.mutex.Unlock()
	// fmt.Printf("timer stop end: %p %p\n", t, t.timer)
}
*/

func (t *Timer) StartIn(secs float64, perform func()) {
	t.Stop()
	t.timer = time.AfterFunc(ztime.SecondsDur(secs), func() {
		t.timer = nil
		defer zlog.HandlePanic(true)
		perform()
	})
}

func (t *Timer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}

func (t *Timer) IsRunning() bool {
	return t.timer != nil
}

