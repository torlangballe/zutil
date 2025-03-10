package zmath

import (
	"math"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
)

type BucketType string

// BucketFilter accepts pos+values, aggregating all that are within a repeating pos period
// It assumes the positions are coming in order.
// With a BucketNearest type, it aggregates on the pos nearest the middle of the period, storing it's value too.
type BucketFilter struct {
	BucketResult
	Type    BucketType
	GotFunc func(result BucketResult, periodIndex int)

	Period   float64
	StartPos float64
}

type BucketResult struct {
	CurrentCellPos     float64
	BestVal            float64
	ValueSum           float64
	BestPos            float64
	BestPayload        any
	FirstPayload       any
	LastPayload        any
	MaxVal             float64
	MaxPayload         any
	MinPayload         any
	MinVal             float64
	Count              int
	BestIndex          int                                              // how far into Count inputs BestVal is
	IsFlushedWithin    bool                                             // true if this result is due to a outside-forced flush
	IsBestOverrideFunc func(f *BucketFilter, payload any) zbool.BoolInd // if not nil and returns true, set best for current payload, if false don't. undef is use normal method
	Histogram          *Histogram
}

const (
	BucketNearest   BucketType = "nearest"
	BucketLargest   BucketType = "largest"
	BucketHistogram BucketType = "histogram"
)

func NewBucketFilter(start, period float64, t BucketType) *BucketFilter {
	f := &BucketFilter{}
	f.Type = t
	f.StartPos = start
	if f.Type == BucketHistogram {
		f.Histogram = &Histogram{}
	}
	// zlog.Info("NewBucket:", zlog.Pointer(f), start, period)
	f.Period = period
	f.CurrentCellPos = start
	f.BestPayload = nil
	return f
}

func (f *BucketFilter) Flush() {
	f.FlushWithEndPos(zfloat.Undefined)
}

func (f *BucketFilter) FlushWithEndPos(atPos float64) {
	if f.BestPayload != nil {
		f.IsFlushedWithin = (atPos == zfloat.Undefined || atPos < f.CurrentCellPos+f.Period)
		periodIndex := int((f.CurrentCellPos - f.StartPos) / f.Period)
		if f.GotFunc != nil {
			f.GotFunc(f.BucketResult, periodIndex)
		}
		f.BestPayload = nil
		f.LastPayload = nil
		if f.Type == BucketHistogram {
			clear(f.Histogram.Classes)
			f.Histogram.OutlierAbove = 0
			f.Histogram.OutlierBelow = 0
		}
	} else {
		// zlog.Info("NoFlush:", zlog.Pointer(f))
	}
}

func (f *BucketFilter) BucketStartForPos(pos float64) float64 {
	return f.StartPos + RoundToModF64(pos-f.StartPos, f.Period)
}

func (f *BucketFilter) aggregate(payload any, pos, val float64) {
	f.LastPayload = payload
	f.Count++
	f.ValueSum += val
	if f.BestPayload == nil {
		f.ValueSum = val
		f.Count = 1
		f.MinVal = val
		f.MaxVal = val
		f.FirstPayload = payload
		f.MaxPayload = payload
		f.MinPayload = payload
		f.BestPayload = payload
		f.BestPos = pos
		f.BestVal = val
		f.CurrentCellPos = f.BucketStartForPos(pos)
		// zlog.Info("aggregate new:", val, time.UnixMicro(int64(pos)), payload)
		if f.Type == BucketHistogram {
			f.Histogram.Add(val)
		}
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
	if f.IsBestOverrideFunc != nil {
		best := f.IsBestOverrideFunc(f, payload)
		if !best.IsUnknown() {
			add = best.IsTrue()
			if !add {
				return
			}
		}
	}
	if !add || f.Type == BucketHistogram {
		switch f.Type {
		case BucketNearest:
			mid := f.CurrentCellPos + f.Period/2
			add = (math.Abs(f.BestPos-mid) > math.Abs(pos-mid))
		case BucketLargest:
			add = (val > f.BestVal)
		case BucketHistogram:
			f.Histogram.Add(val)
			add = true
		}
	}
	if add {
		f.BestIndex = f.Count - 1
		f.BestPos = pos
		f.BestVal = val
		f.BestPayload = payload
		// zlog.Info("aggregate1:", f.Count, f.BestIndex, f.CurrentCellPos)
	}
}

func (f *BucketFilter) Set(payload any, pos, val float64) {
	// zlog.Info("buck.Set:", zlog.Pointer(f), time.UnixMicro(int64(pos)), val)
	if pos < f.CurrentCellPos {
		zlog.Error("val before start:", payload, pos, f.CurrentCellPos)
		return
	}
	if pos >= f.CurrentCellPos+f.Period {
		// zlog.Info("buck.Flush from set:", zlog.Pointer(f), pos)
		f.FlushWithEndPos(pos)
	}
	f.aggregate(payload, pos, val)
}

func (f *BucketFilter) SetValueInPosRange(payload any, posStart, posEnd, val float64) {
	// zlog.Info("buck.Set:", zlog.Pointer(f), time.UnixMicro(int64(pos)), val)
	if posStart < f.CurrentCellPos || posEnd < f.CurrentCellPos {
		zlog.Error("val before start:", payload, posStart, posEnd, f.CurrentCellPos)
		return
	}
	for pos := posStart; pos <= posEnd; pos += f.Period {
		f.Set(payload, pos, val)
	}
}
