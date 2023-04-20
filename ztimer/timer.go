package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"errors"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
)

type Timer struct {
	timer *time.Timer
	mutex sync.Mutex
	secs  float64
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

var (
	timersCount = map[float64]int{}
	countMutex  sync.Mutex
)

func (t *Timer) StartIn(secs float64, perform func()) {
	t.Stop()
	countMutex.Lock()
	timersCount[secs]++
	t.secs = secs
	if timersCount[secs]%1000 == 999 {
		zlog.Error(nil, timersCount[secs], "timers of", secs, "seconds started", zlog.CallingStackString())
	}
	countMutex.Unlock()

	t.timer = time.AfterFunc(secs2Dur(secs), func() {
		countMutex.Lock()
		timersCount[secs]--
		countMutex.Unlock()
		t.timer = nil
		defer zlog.HandlePanic(true)
		perform()
	})
}

func (t *Timer) Stop() {
	if t.timer != nil {
		countMutex.Lock()
		timersCount[t.secs]--
		countMutex.Unlock()
		t.timer.Stop()
		t.timer = nil
	}
}

func (t *Timer) IsRunning() bool {
	return t.timer != nil
}

// TryFor waits for seecs for try function to run, then continues.
// Note: it doesn't STOP the function somehow.
func TryFor(secs float64, try func()) (err error) {
	timer := TimerNew()
	timer.StartIn(secs, func() {
		err = errors.New("try failed")
	})
	try()
	timer.Stop()
	return
}

func StartAt(t time.Time, f func()) *Timer {
	secs := dur2Secs(time.Until(t))
	timer := StartIn(secs, f)
	return timer
}
