package zprocess

import (
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

func testPooling(t *testing.T) {
	zlog.Warn("testPooling")

	start := time.Now()
	var all = []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	PoolWorkOnItems[int](all, 3, func(t *int) {
		time.Sleep(time.Millisecond * 100)
	})
	since := ztime.Since(start)
	if since < 0.4 || since > 0.45 {
		t.Error("Duration not ~400ms")
		return
	}
}

func TestAll(t *testing.T) {
	testPooling(t)
}
