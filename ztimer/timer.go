package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/ztime"
)

type Timer struct {
	timer *time.Timer
	mutex sync.Mutex
}

func TimerNew() *Timer {
	return &Timer{}
}

func StartIn(secs float64, onMainThread bool, perform func()) *Timer {
	t := TimerNew()
	t.StartIn(secs, onMainThread, perform)
	return t
}

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

func (t *Timer) StartIn(secs float64, onMainThread bool, perform func()) *Timer {
	// fmt.Println("timer start1:")
	t.Stop()
	t.timer = time.NewTimer(ztime.SecondsDur(secs))
	go t.check(perform)
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
