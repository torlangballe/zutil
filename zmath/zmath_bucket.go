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
	GotFunc func(payload, lastPayload any, pos, periodPos, val float64, periodIndex int)

	period         float64
	startPos       float64
	currentCellPos float64
	bestVal        float64
	bestPos        float64
	bestPayload    any
	lastPayload    any
}

func NewBucketFilter(start, period float64) *BucketFilter {
	f := &BucketFilter{}
	f.Type = BucketNearest
	f.startPos = start
	f.period = period
	f.currentCellPos = start
	f.bestPayload = nil
	return f
}

func (f *BucketFilter) Flush() {
	if f.bestPayload != nil {
		periodIndex := int((f.currentCellPos - f.startPos) / f.period)
		// zlog.Info("Flush:", zlog.Pointer(f), periodIndex, f.bestPayload != nil, "current:", time.UnixMicro(int64(f.currentCellPos)), "best:", time.UnixMicro(int64(f.bestPos)))
		f.GotFunc(f.bestPayload, f.lastPayload, f.bestPos, f.currentCellPos, f.bestVal, periodIndex)
		f.bestPayload = nil
		f.lastPayload = nil
	} else {
		// zlog.Info("NoFlush:", zlog.Pointer(f))
	}
}

func (f *BucketFilter) aggregate(payload any, pos, val float64) {
	f.lastPayload = payload
	// zlog.Info("aggregate1:", zlog.Pointer(f), time.UnixMicro(int64(pos)), f.bestPayload != nil)
	if f.bestPayload == nil {
		f.bestPayload = payload
		f.bestPos = pos
		f.bestVal = val
		f.currentCellPos = f.startPos + RoundToModF64(pos-f.startPos, f.period)
		// zlog.Info("aggregate new:", zlog.Pointer(f), time.UnixMicro((int64(pos))), "current:", time.UnixMicro((int64(f.currentCellPos))), time.Duration(pos-f.currentCellPos), time.Duration(f.period)*time.Microsecond)
		return
	}
	switch f.Type {
	case BucketNearest:
		mid := f.currentCellPos + f.period/2
		if math.Abs(f.bestPos-mid) > math.Abs(pos-mid) {
			f.bestPos = pos
			f.bestVal = val
			f.bestPayload = payload
		}
	}
}

func (f *BucketFilter) Set(payload any, pos, val float64) {
	// zlog.Info("buck.Set:", zlog.Pointer(f), time.UnixMicro(int64(pos)), val)
	if pos < f.currentCellPos {
		zlog.Error("val before start:", payload, pos, f.currentCellPos)
		return
	}
	if pos >= f.currentCellPos+f.period {
		f.Flush()
	}
	f.aggregate(payload, pos, val)
}
