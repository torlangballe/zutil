package ztimer

//  Created by Tor Langballe on /18/11/15.

import (
	"time"

	"github.com/torlangballe/zutil/ztime"
)

type Timer struct {
	timer *time.Timer
}

func TimerNew() *Timer {
	return &Timer{}
}

func StartIn(secs float64, onMainThread bool, perform func()) *Timer {
	t := TimerNew()
	t.StartIn(secs, onMainThread, perform)
	return t
}

func (t *Timer) StartIn(secs float64, onMainThread bool, perform func()) *Timer {
	// fmt.Println("timer set:", secs, zlog.GetCallingStackString(3, "\n"))
	t.Stop()
	t.timer = time.NewTimer(ztime.SecondsDur(secs))
	go func() {
		<-t.timer.C
		perform()
	}()
	return t
}

func (t *Timer) Stop() {
	if t.timer != nil {
		t.timer.Stop()
		t.timer = nil
	}
}
