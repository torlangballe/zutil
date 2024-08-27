package ztesting

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zlog"
)

func Compare[N comparable](t *testing.T, str string, n ...N) bool {
	var fail bool
	for i := 0; i < len(n); i += 2 {
		c := n[i]
		val := n[i+1]
		if c != val {
			str += fmt.Sprint(" ", c, " != ", val)
			fail = true
		}
	}
	if fail {
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
	}
	return !fail
}
