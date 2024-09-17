package ztesting

import (
	"cmp"
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

func Equal[N comparable](t *testing.T, a, b N, parts ...any) bool {
	if a != b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%s' != '%s'", a, b)
		zlog.Error(zlog.StackAdjust(1), zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func Different[N comparable](t *testing.T, a, b N, parts ...any) bool {
	if a == b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%s' == '%s'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func GreaterThan[N cmp.Ordered](t *testing.T, a, b N, parts ...any) bool {
	if a < b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%s' < '%s'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func LessThan[N cmp.Ordered](t *testing.T, a, b N, parts ...any) bool {
	if a > b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%s' > '%s'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}
