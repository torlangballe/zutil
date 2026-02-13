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
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%v' != '%v'", a, b)
		zlog.Error(zlog.StackAdjust(1), zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func MultiEqual[N comparable](t *testing.T, a N, bs ...N) bool {
	for _, b := range bs {
		if a == b {
			return true
		}
	}
	str := fmt.Sprintf(" '%v' != '%v'", a, bs)
	zlog.Error(zlog.StackAdjust(1), zlog.StackAdjust(1), "Fail:", str)
	t.Error(str)
	return false
}

func Different[N comparable](t *testing.T, a, b N, parts ...any) bool {
	if a == b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%v' == '%v'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func GreaterThan[N cmp.Ordered](t *testing.T, a, b N, parts ...any) bool {
	if a < b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%v' < '%v'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func LessThan[N cmp.Ordered](t *testing.T, a, b N, parts ...any) bool {
	if a > b {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" '%v' > '%v'", a, b)
		zlog.Error(zlog.StackAdjust(1), "Fail:", str)
		t.Error(str)
		return false
	}
	return true
}

func OnError(t *testing.T, err error, parts ...any) bool {
	if err != nil {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" Error: %v", err)
		zlog.Error(zlog.StackAdjust(1), "OnError:", str)
		t.Error(str)
		return true
	}
	return false
}

func OnErrorFatal(t *testing.T, err error, parts ...any) {
	if err != nil {
		str := zstr.Spaced(parts...) + fmt.Sprintf(" Error: %v", err)
		zlog.Error(zlog.StackAdjust(1), "OnError:", str)
		t.Fatal(str)
	}
}
