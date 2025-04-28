package zhistogram

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

func testSimple(t *testing.T) {
	zlog.Warn("testSimple")
	h1 := New(10, 0, 70)
	h1.Add(5)
	h1.Add(10)
	h1.Add(12)
	h1.Add(55)
	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[{1 } {2 } {0 } {0 } {0 } {1 } {0 }]", "classes not equal")

	h2 := New(10, 0, 70)
	h2.Add(7)
	h2.Add(14)
	h2.Add(42)
	ztesting.Equal(t, fmt.Sprint(h2.Classes), "[{1 } {1 } {0 } {0 } {1 } {0 } {0 }]", "classes2 not equal")

	h1.MergeIn(*h2)

	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[{2 } {3 } {0 } {0 } {1 } {1 } {0 }]", "merged not equal")
}

func testNames(t *testing.T) {
	zlog.Warn("testNames")
	h1 := New(-1, 0, 0)
	h1.AddName("512")
	h1.AddName("512")
	h1.AddName("512")
	h1.AddName("256")
	h1.AddName("256")
	h1.AddName("64")
	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[{3 512} {2 256} {1 64}]", "set-count not equal")
	// ztesting.Equal(t, fmt.Sprint(h1.ValueSet), "[512 256 64]", "sets not equal")

	h2 := New(-1, 0, 0)
	h2.AddName("16")
	h2.AddName("512")
	h2.AddName("512")
	h2.AddName("256")
	h2.AddName("256")
	h2.AddName("256")
	h2.AddName("64")
	h2.AddName("64")
	h2.AddName("64")
	h2.AddName("64")
	h2.AddName("64")
	ztesting.Equal(t, fmt.Sprint(h2.Classes), "[{1 16} {2 512} {3 256} {5 64}]", "set-count not equal")

	h1.MergeIn(*h2)

	ztesting.Equal(t, fmt.Sprint(h1.Classes), "[{5 512} {5 256} {6 64} {1 16}]", "set-merged-count not equal")
}

func TestAll(t *testing.T) {
	testSimple(t)
	testNames(t)
}
