package zmath

import (
	"math"

	"github.com/torlangballe/zutil/zlog"
)

type BucketType string

const (
	BucketNearest BucketType = "nearest"
	BucketMax     BucketType = "max"
)

type BucketResult struct {
	CurrentCellPos float64
	BestVal        float64
	BestPos        float64
	BestPayload    any
	FirstPayload   any
	LastPayload    any
	MaxVal         float64
	MaxPayload     any
	MinPayload     any
	MinVal         float64
	Count          int
	BestIndex      int // how far into Count inputs BestVal is
}

// BucketFilter accepts pos+values, aggregating all that are within a repeating pos period
// It assumes the positions are coming in order.
// With a BucketNearest type, it aggregates on the pos nearest the middle of the period, storing it's value too.
type BucketFilter struct {
	BucketResult
	Type    BucketType
	GotFunc func(result BucketResult, periodIndex int)

	period   float64
	startPos float64
}

func NewBucketFilter(start, period float64) *BucketFilter {
	f := &BucketFilter{}
	f.Type = BucketNearest
	f.startPos = start
	// zlog.Info("NewBucket:", zlog.Pointer(f), start, period)
	f.period = period
	f.CurrentCellPos = start
	f.BestPayload = nil
	return f
}

func (f *BucketFilter) Flush() {
	if f.BestPayload != nil {
		periodIndex := int((f.CurrentCellPos - f.startPos) / f.period)
		// zlog.Info("Flush:", zlog.Pointer(f), periodIndex, f.BestPayload != nil, "current:", time.UnixMicro(int64(f.CurrentCellPos)), "best:", time.UnixMicro(int64(f.bestPos)))
		f.GotFunc(f.BucketResult, periodIndex)
		f.BestPayload = nil
		f.LastPayload = nil
	} else {
		// zlog.Info("NoFlush:", zlog.Pointer(f))
	}
}

func (f *BucketFilter) aggregate(payload any, pos, val float64) {
	f.LastPayload = payload
	f.Count++
	// zlog.Info("aggregate1:", zlog.Pointer(f), time.UnixMicro(int64(pos)), f.BestPayload != nil)
	if f.BestPayload == nil {
		f.Count = 0
		f.MinVal = val
		f.MaxVal = val
		f.FirstPayload = payload
		f.MaxPayload = payload
		f.MinPayload = payload
		f.BestPayload = payload
		f.BestPos = pos
		f.BestVal = val
		f.CurrentCellPos = f.startPos + RoundToModF64(pos-f.startPos, f.period)
		// zlog.Info("aggregate new:", f.startPos, zlog.Pointer(f), time.UnixMicro((int64(pos))), "current:", time.UnixMicro((int64(f.currentCellPos))), time.Duration(pos-f.CurrentCellPos), time.Duration(f.period)*time.Microsecond)
		return
	}
	if f.MinVal > val {
		f.MinVal = val
		f.MinPayload = payload
	}
	if f.MaxVal < val {
		f.MaxVal = val
		f.MaxPayload = payload
	}
	var add bool
	switch f.Type {
	case BucketMax:
		add = (val > f.MaxVal)
	case BucketNearest:
		mid := f.CurrentCellPos + f.period/2
		add = (math.Abs(f.BestPos-mid) > math.Abs(pos-mid))
	}
	if add {
		f.BestIndex = f.Count - 1
		f.BestPos = pos
		f.BestVal = val
		f.BestPayload = payload
	}
}

func (f *BucketFilter) Set(payload any, pos, val float64) {
	// zlog.Info("buck.Set:", zlog.Pointer(f), time.UnixMicro(int64(pos)), val)
	if pos < f.CurrentCellPos {
		zlog.Error("val before start:", payload, pos, f.CurrentCellPos)
		return
	}
	if pos >= f.CurrentCellPos+f.period {
		// zlog.Info("buck.Flush from set:", zlog.Pointer(f), pos)
		f.Flush()
	}
	f.aggregate(payload, pos, val)
}
