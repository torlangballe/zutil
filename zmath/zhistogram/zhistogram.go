package zhistogram

import (
	"slices"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zslice"
)

type Class struct {
	Count int
	Label string
}

type Histogram struct {
	Step  float64        `json:",omitempty"` // Step is interval of each bar
	Range zmath.RangeF64 `json:",omitempty"` // Range Max is 10 if you want values from uo to 10.999999...

	Classes      []Class `json:",omitempty"`
	OutlierBelow int     `json:",omitempty"`
	OutlierAbove int     `json:",omitempty"`

	IsNames bool `json:",omitempty"`
}

// Setup sets up the histogram with a step and min/max range.
// It calculates the number of classes based on these parameters.
func (h *Histogram) Setup(step, min, max float64) {
	max = min + zmath.RoundUpToModF64(max-min, step)
	// zlog.Info("Hist.Setup:", step, min, max)
	if step == -1 {
		h.Step = step
		h.IsNames = true
		return
	}
	h.Step = step
	h.Range.Min = min
	h.Range.Max = max
	h.Range.Valid = true
	n := int(h.Range.Length() / h.Step)
	h.Classes = make([]Class, n)
}

func New(step, min, max float64) *Histogram {
	h := &Histogram{}
	h.Setup(step, min, max)
	return h
}

func (h *Histogram) FindName(name string) int {
	for i, c := range h.Classes {
		if c.Label == name {
			return i
		}
	}
	return -1
}

func (h *Histogram) AddName(name string) {
	zlog.Assert(h.IsNames)
	i := h.FindName(name)
	if i == -1 {
		c := Class{Count: 1, Label: name}
		h.Classes = append(h.Classes, c)
		return
	}
	h.Classes[i].Count++
	// zlog.Info("AddSet:", name, h.Classes)
	return
}

func (h *Histogram) Add(value float64) {
	zlog.Assert(!h.IsNames)
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
	h.Classes[i].Count++
}

func (h *Histogram) TotalCount() int {
	count := h.OutlierAbove + h.OutlierBelow
	for _, c := range h.Classes {
		count += c.Count
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
	zlog.Assert(h.IsNames == in.IsNames, h.IsNames, in.IsNames)

	if h.IsNames {
		inSet := in.Classes
		for i, c := range h.Classes {
			fi := slices.IndexFunc(inSet, func(e Class) bool {
				return e.Label == c.Label
			})
			if fi != -1 {
				h.Classes[i].Count += inSet[fi].Count
				zslice.RemoveAt(&inSet, fi)
			}
		}
		for _, is := range inSet {
			h.Classes = append(h.Classes, is)
		}
	} else {
		more := len(in.Classes) - len(h.Classes)
		for i := 0; i < more; i++ {
			h.Classes = append(h.Classes, Class{})
		}
		for i, c := range in.Classes {
			h.Classes[i].Count += c.Count
		}
	}
	h.OutlierAbove += in.OutlierAbove
	h.OutlierBelow += in.OutlierBelow
}

func (h *Histogram) MaxUsedClassIndex() int {
	for i := len(h.Classes) - 1; i >= 0; i-- {
		if h.Classes[i].Count != 0 {
			return i + 1
		}
	}
	return 0
}
