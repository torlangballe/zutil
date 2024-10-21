package ztimer

import (
	"testing"
	"time"

	"github.com/torlangballe/zutil/ztesting"
)

//  Created by Tor Langballe on /18/11/15.

var str string

func TestResetRepeat(t *testing.T) {
	r := RepeaterNew()
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
