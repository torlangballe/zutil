package zbucket

import (
	"math"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zmath/zhistogram"
)

type Type string

// Filter accepts pos+values, aggregating all that are within a repeating pos period
// It assumes the positions are coming in order.
// With a Nearest type, it aggregates on the pos nearest the middle of the period, storing it's value too.
type Filter struct {
	Result
	Type    Type
	GotFunc func(result Result, periodIndex int)

	Period   float64
	StartPos float64
}

type Result struct {
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
	IsBestOverrideFunc func(f *Filter, payload any) zbool.BoolInd // if not nil and returns true, set best for current payload, if false don't. undef is use normal method
	Histogram          *zhistogram.Histogram
}

const (
	Nearest   Type = "nearest"
	Largest   Type = "largest"
	Histogram Type = "histogram"
)

func NewFilter(start, period float64, t Type) *Filter {
	f := &Filter{}
	f.Type = t
	f.StartPos = start
	if f.Type == Histogram {
		f.Histogram = &zhistogram.Histogram{}
	}
	// zlog.Info("New:", zlog.Pointer(f), start, period)
	f.Period = period
	f.CurrentCellPos = start
	f.BestPayload = nil
	return f
}

func (f *Filter) Flush() {
	f.FlushWithEndPos(zfloat.Undefined)
}

func (f *Filter) FlushWithEndPos(atPos float64) {
	if f.BestPayload != nil {
		f.IsFlushedWithin = (atPos == zfloat.Undefined || atPos < f.CurrentCellPos+f.Period)
		periodIndex := int((f.CurrentCellPos - f.StartPos) / f.Period)
		if f.GotFunc != nil {
			f.GotFunc(f.Result, periodIndex)
		}
		f.BestPayload = nil
		f.LastPayload = nil
		if f.Type == Histogram {
			clear(f.Histogram.Classes)
			f.Histogram.OutlierAbove = 0
			f.Histogram.OutlierBelow = 0
		}
	} else {
		// zlog.Info("NoFlush:", zlog.Pointer(f))
	}
}

func (f *Filter) StartForPos(pos float64) float64 {
	return f.StartPos + zmath.RoundToModF64(pos-f.StartPos, f.Period)
}

func (f *Filter) aggregate(payload any, pos, val float64) {
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
		f.CurrentCellPos = f.StartForPos(pos)
		// zlog.Info("aggregate new:", val, time.UnixMicro(int64(pos)), payload)
		if f.Type == Histogram {
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
	if !add || f.Type == Histogram {
		switch f.Type {
		case Nearest:
			mid := f.CurrentCellPos + f.Period/2
			add = (math.Abs(f.BestPos-mid) > math.Abs(pos-mid))
		case Largest:
			add = (val > f.BestVal)
		case Histogram:
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

func (f *Filter) Set(payload any, pos, val float64) {
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

func (f *Filter) SetValueInPosRange(payload any, posStart, posEnd, val float64) {
	// zlog.Info("buck.Set:", zlog.Pointer(f), time.UnixMicro(int64(pos)), val)
	if posStart < f.CurrentCellPos || posEnd < f.CurrentCellPos {
		zlog.Error("val before start:", payload, posStart, posEnd, f.CurrentCellPos)
		return
	}
	for pos := posStart; pos <= posEnd; pos += f.Period {
		f.Set(payload, pos, val)
	}
}
