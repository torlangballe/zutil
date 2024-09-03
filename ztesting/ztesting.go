package ztesting

import (
	"cmp"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

func Equal[N comparable](t *testing.T, str string, a, b N) bool {
	if a != b {
		str := zstr.Spaced(str, a, "!=", b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func Different[N comparable](t *testing.T, str string, a, b N) bool {
	if a == b {
		str := zstr.Spaced(str+":", a, "==", b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func GreaterThan[N cmp.Ordered](t *testing.T, str string, a, b N) bool {
	if a < b {
		str := zstr.Spaced(str+":", a, "<", b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func LessThan[N cmp.Ordered](t *testing.T, str string, a, b N) bool {
	if a > b {
		str := zstr.Spaced(str+":", a, ">", b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}
