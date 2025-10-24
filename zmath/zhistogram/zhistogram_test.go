package zhistogram

import (
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

func testSimple(t *testing.T) {
	zlog.Warn("testSimple")
	h1 := New()
	h1.Setup(10, 0, 70)
	h1.Add(5)
	h1.Add(10)
	h1.Add(12)
	h1.Add(55)
	ztesting.Equal(t, h1.ClassString(), "10:2 20:1 30:0 40:0 50:0 60:1 70:0", "classes not equal")

	h2 := New()
	h2.Setup(10, 0, 70)
	h2.Add(7)
	h2.Add(14)
	h2.Add(42)
	ztesting.Equal(t, h2.ClassString(), "10:1 20:1 30:0 40:0 50:1 60:0 70:0", "classes2 not equal")

	h1.MergeIn(*h2)

	ztesting.Equal(t, h1.ClassString(), "10:3 20:2 30:0 40:0 50:1 60:1 70:0", "merged not equal")
}

func testNames(t *testing.T) {
	zlog.Warn("testNames")
	h1 := New()
	h1.AccumilateClasses = true
	h1.Add(1)
	h1.Add(1)
	h1.Add(2)
	h1.Add(2)
	h1.Add(2)
	h1.Add(2)
	h1.Add(2)
	h1.Add(3)
	ztesting.Equal(t, h1.ClassString(), "1:2 2:5 3:1", "fruit-count not equal")

	/*
		h2 := New()
		h2.SetupNamedRanges(0, 60, "minute", 3600, "hour", 3600*24, "day")
		h2.Add(20)
		h2.Add(50)
		h2.Add(200)
		h2.Add(2220)
		h2.Add(7000)
		ztesting.Equal(t, h2.ClassString(), "minute/60:2 hour/3600:2 day/86400:1", "time-count not equal")

		h3 := New()
		h3.SetupNamedRanges(0, 60, "minute", 3600, "hour", 3600*24, "day")
		h3.Add(200)

		h2.MergeIn(*h3)

		ztesting.Equal(t, h2.ClassString(), "minute/60:2 hour/3600:3 day/86400:1", "set-merged-count not equal")
	*/
}

func TestAll(t *testing.T) {
	testSimple(t)
	testNames(t)
}
