package zmath

import (
	"github.com/torlangballe/zutil/zlog"
)

type Histogram struct {
	Step         float64  `json:",omitempty"` // Step is interval of each bar
	Range        RangeF64 `json:",omitempty"` // Range Max is 10 if you want values from uo to 10.999999...
	Classes      []int    `json:",omitempty"`
	OutlierBelow int      `json:",omitempty"`
	OutlierAbove int      `json:",omitempty"`
}

func (h *Histogram) Setup(step, min, max float64) {
	max = min + RoundUpToModF64(max-min, step)
	h.Step = step
	h.Range.Min = min
	h.Range.Max = max
	h.Range.Valid = true
	n := int(h.Range.Length() / h.Step)
	h.Classes = make([]int, n+1)
}

func (h *Histogram) Add(value float64) {
	if value < h.Range.Min {
		h.OutlierBelow++
		return
	}
	if value >= h.Range.Max+h.Step {
		h.OutlierAbove++
		return
	}
	i := int((value - h.Range.Min) / h.Step)
	zlog.Assert(i < len(h.Classes), i, len(h.Classes))
	h.Classes[i]++
}

func (h *Histogram) TotalCount() int {
	count := h.OutlierAbove + h.OutlierBelow
	for _, c := range h.Classes {
		count += c
	}
	return count
}
