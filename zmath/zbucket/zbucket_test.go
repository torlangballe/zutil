package zbucket

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath/zhistogram"
	"github.com/torlangballe/zutil/ztesting"
)

func TestHistogram(t *testing.T) {
	fmt.Println("TestHistogram")

	b := NewFilter(0, 100, Histogram)
	b.Histogram.Setup(1, 0, 10)

	dummy := "dummy"
	tpos := 20.0
	b.Set(dummy, tpos, 9)
	b.Set(dummy, tpos, 9)
	b.Set(dummy, tpos, 9)
	ztesting.Equal(t, len(b.Histogram.Classes), 10)
	ztesting.Equal(t, b.Histogram.TotalCount(), 3)
	b.Set(dummy, tpos, 5)
	b.Set(dummy, tpos, 5)
	zlog.Warn("Classes:", b.Histogram.Classes)
	ztesting.Equal(t, b.Histogram.TotalCount(), 5)
	ztesting.Equal(t, b.Histogram.Classes[9], zhistogram.Class{3, ""})
	ztesting.Equal(t, b.Histogram.Classes[5], zhistogram.Class{2, ""})

	b.Set(dummy, tpos, 10)
	b.Set(dummy, tpos, 25)
	ztesting.Equal(t, b.Histogram.OutlierAbove, 2)
	b.Set(dummy, tpos, 11)
	ztesting.Equal(t, b.Histogram.OutlierAbove, 3)
	b.Set(dummy, tpos, -1)
	ztesting.Equal(t, b.Histogram.OutlierBelow, 1)
	ztesting.Equal(t, b.Histogram.TotalCount(), 9)

	b.Set(dummy, tpos+100, 5)
	ztesting.Equal(t, b.Histogram.TotalCount(), 1)

	b.Flush()
	ztesting.Equal(t, b.Histogram.TotalCount(), 0)
}
