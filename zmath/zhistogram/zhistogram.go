package zhistogram

import (
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
)

type Histogram struct {
	Step         float64        `json:",omitempty"` // Step is interval of each bar
	Range        zmath.RangeF64 `json:",omitempty"` // Range Max is 10 if you want values from uo to 10.999999...
	Classes      []int          `json:",omitempty"`
	OutlierBelow int            `json:",omitempty"`
	OutlierAbove int            `json:",omitempty"`
}

// Setup sets up the histogram with a step and min/max range.
// It calculates the number of classes based on these parameters.
func (h *Histogram) Setup(step, min, max float64) {
	max = min + zmath.RoundUpToModF64(max-min, step)
	h.Step = step
	h.Range.Min = min
	h.Range.Max = max
	h.Range.Valid = true
	n := int(h.Range.Length() / h.Step)
	h.Classes = make([]int, n)
}

func (h *Histogram) Add(value float64) {
	if value < h.Range.Min {
		h.OutlierBelow++
		return
	}
	if value >= h.Range.Max {
		h.OutlierAbove++
		return
	}
	i := int((value - h.Range.Min) / h.Step)
	zlog.Assert(i < len(h.Classes), value, h.Step, i, len(h.Classes))
	h.Classes[i]++
}

func (h *Histogram) TotalCount() int {
	count := h.OutlierAbove + h.OutlierBelow
	for _, c := range h.Classes {
		count += c
	}
	return count
}

func (h *Histogram) CountAsRatio(c int) float64 {
	total := h.TotalCount()
	return float64(c) / float64(total)
}

func (h *Histogram) CountAsPercent(c int) int {
	return int(100 * h.CountAsRatio(c))
}

func (h *Histogram) MergeIn(in Histogram) {
	zlog.Assert(h.Step == in.Step, h.Step, in.Step)
	zlog.Assert(h.Range == in.Range, h.Range, in.Range)
	more := len(in.Classes) - len(h.Classes)
	for i := 0; i < more; i++ {
		h.Classes = append(h.Classes, 0)
	}
	for i, ci := range in.Classes {
		h.Classes[i] += ci
	}
	h.OutlierAbove += in.OutlierAbove
	h.OutlierBelow += in.OutlierBelow
}

func (h *Histogram) MaxUsedClassIndex() int {
	for i := len(h.Classes) - 1; i >= 0; i-- {
		if h.Classes[i] != 0 {
			return i + 1
		}
	}
	return 0
}
