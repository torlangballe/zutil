package ztimer

import (
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

//  Created by Tor Langballe on /18/11/15.

var str string

func TestResetRepeat(t *testing.T) {
	r := NewRepeater()
	r.Set(0.1, false, func() bool {
		str = "first"
		return false
	})
	time.Sleep(time.Millisecond * 50)
	r.Set(0.1, false, func() bool {
		str = "second"
		return false
	})
	time.Sleep(time.Millisecond * 120)
	ztesting.Equal(t, str, "second", "second timer kick in")
}

func TestStop(t *testing.T) {
	var fired = new(bool)
	timer := RepeatForever(1, func() {
		zlog.Warn("Fired")
		*fired = true
	})
	time.Sleep(time.Millisecond * 100)
	timer.Stop()
	if *fired {
		t.Error("Should not have fired")
	}
}
