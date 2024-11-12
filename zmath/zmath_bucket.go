package zmath

import (
	"math"

	"github.com/torlangballe/zutil/zlog"
)

type BucketType string

const (
	BucketNearest BucketType = "nearest"
)

// BucketFilter accepts pos+values, aggregating all that are within a repeating pos period
// It assumes the positions are coming in order.
// With a BucketNearest type, it aggregates on the pos nearest the middle of the period, storing it's value too.
type BucketFilter struct {
	Type    BucketType
	GotFunc func(id int, pos, periodPos, val float64, periodIndex int)

	period         float64
	startPos       float64
	currentCellPos float64
	bestVal        float64
	bestPos        float64
	bestID         int
}

func NewBucketFilter(start, period float64) *BucketFilter {
	f := &BucketFilter{}
	f.Type = BucketNearest
	f.startPos = start
	f.period = period
	f.currentCellPos = start
	f.bestID = -1
	return f
}

func (f *BucketFilter) Flush() {
	if f.bestID != -1 {
		periodIndex := int((f.currentCellPos - f.startPos) / f.period)
		// zlog.Info("Flush:", periodIndex, f.bestID, "current:", time.UnixMicro(int64(f.currentCellPos)), "best:", time.UnixMicro(int64(f.bestPos)))
		f.GotFunc(f.bestID, f.bestPos, f.currentCellPos, f.bestVal, periodIndex)
		f.bestID = -1
	}
}

func (f *BucketFilter) aggregate(id int, pos, val float64) {
	if f.bestID == -1 {
		f.bestID = id
		f.bestPos = pos
		f.bestVal = val
		f.currentCellPos = f.startPos + RoundToModF64(pos-f.startPos, f.period)
		// zlog.Info("aggregate:", time.UnixMicro((int64(pos))), "current:", time.UnixMicro((int64(f.currentCellPos))), time.Duration(pos-f.currentCellPos), time.Duration(f.period)*time.Microsecond)
		return
	}
	switch f.Type {
	case BucketNearest:
		mid := f.currentCellPos + f.period/2
		if math.Abs(f.bestPos-mid) > math.Abs(pos-mid) {
			f.bestPos = val
			f.bestVal = val
			f.bestID = id
		}
	}
}

func (f *BucketFilter) Set(id int, pos, val float64) {
	if pos < f.currentCellPos {
		zlog.Error("val before start:", id, pos, f.currentCellPos)
		return
	}
	if pos >= f.currentCellPos+f.period {
		f.Flush()
	}
	f.aggregate(id, pos, val)
}
