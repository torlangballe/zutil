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

type Stopper interface {
	Stop()
}

func TimerNew() *Timer {
	return &Timer{}
}

func StartIn(secs float64, perform func()) *Timer {
	t := TimerNew()
	t.StartIn(secs, perform)
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

func (t *Timer) StartIn(secs float64, perform func()) *Timer {
	// zlog.Info("timer start1:")
	t.Stop()
	t.timer = time.NewTimer(ztime.SecondsDur(secs))
	go func() {
		// defer zlog.LogRecoverAndExit()
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

// create a RateLimiter to only do a function every n seconds since last time it was done for a given id.
// It bases this on the time since the previous function was **started**.
type RateLimiter struct {
	cache         map[int64]time.Time
	lock          sync.Mutex
	frequencySecs float64
}

func NewRateLimiter(secs float64) *RateLimiter {
	r := &RateLimiter{}
	r.cache = map[int64]time.Time{}
	r.frequencySecs = secs
	return r
}

// Do runs the *do* function if secs (or r.frequencySecs if secs == 0) has passed since last time it was done for that id
func (r *RateLimiter) Do(id int64, secs float64, do func()) {
	if secs == 0 {
		do()
		return
	}
	r.lock.Lock()
	t := r.cache[id]
	freq := r.frequencySecs
	if secs != -1 {
		freq = secs
	}
	ready := (t.IsZero() || ztime.Since(t) > freq)
	if ready {
		r.cache[id] = time.Now()
	}
	r.lock.Unlock()
	if ready {
		do()
	}
}
