package zhistogram

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

func testSimple(t *testing.T) {
	zlog.Warn("TestSimple")
	h1 := New(10, 0, 70)
	h1.Add(5)
	h1.Add(10)
	h1.Add(12)
	h1.Add(55)
	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[1 2 0 0 0 1 0]", "classes not equal")

	h2 := New(10, 0, 70)
	h2.Add(7)
	h2.Add(14)
	h2.Add(42)
	ztesting.Equal(t, fmt.Sprint(h2.Classes), "[1 1 0 0 1 0 0]", "classes2 not equal")

	h1.MergeIn(*h2)

	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[2 3 0 0 1 1 0]", "merged not equal")
}

func testSet(t *testing.T) {
	zlog.Warn("TestSet")
	h1 := New(-1, 0, 0)
	h1.Add(512)
	h1.Add(512)
	h1.Add(512)
	h1.Add(256)
	h1.Add(256)
	h1.Add(64)
	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[3 2 1]", "set-count not equal")
	ztesting.Equal(t, fmt.Sprint(h1.ValueSet), "[512 256 64]", "sets not equal")

	h2 := New(-1, 0, 0)
	h2.Add(16)
	h2.Add(512)
	h2.Add(512)
	h2.Add(256)
	h2.Add(256)
	h2.Add(256)
	h2.Add(64)
	h2.Add(64)
	h2.Add(64)
	h2.Add(64)
	h2.Add(64)
	ztesting.Equal(t, fmt.Sprint(h2.Classes), "[1 2 3 5]", "set-count not equal")
	ztesting.Equal(t, fmt.Sprint(h2.ValueSet), "[16 512 256 64]", "sets not equal")

	h1.MergeIn(*h2)

	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[5 5 6 1]", "set-merged-count not equal")
	ztesting.Equal(t, fmt.Sprint(h1.ValueSet), "[512 256 64 16]", "merged sets not equal")
}

func TestAll(t *testing.T) {
	testSimple(t)
	testSet(t)
}
