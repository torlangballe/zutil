package zhistogram

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zstr"
)

type Class struct {
	Count    int    `json:",omitempty"`
	Label    string `json:",omitempty"`
	MaxRange float64
}

type Histogram struct {
	MinValue          float64
	Classes           []Class `json:",omitempty"`
	OutlierBelow      int     `json:",omitempty"`
	OutlierAbove      int     `json:",omitempty"`
	OutlierBelowSum   float64 `json:",omitempty"`
	OutlierAboveSum   float64 `json:",omitempty"`
	Unit              string  `json:",omitempty"`
	AccumilateClasses bool
}

func New() *Histogram {
	h := &Histogram{}
	return h
}

func (h *Histogram) Copy() Histogram {
	n := *h
	n.Classes = slices.Clone(h.Classes)
	return n
}

// Setup sets up the histogram with a step and min/max range.
// It generates the classes based on these parameters.
func (h *Histogram) Setup(step, min, max float64) {
	h.MinValue = min
	max = min + zmath.RoundUpToModF64(max-min, step)
	for n := min; n < max; n += step {
		class := Class{MaxRange: n + step}
		h.Classes = append(h.Classes, class)
	}
	// zlog.Info("Hist.Setup:", step, min, max)
}

func (h *Histogram) SetupRanges(min float64, maxes ...float64) {
	h.MinValue = min
	for _, m := range maxes {
		class := Class{MaxRange: m}
		h.Classes = append(h.Classes, class)
	}
}

func (h *Histogram) SetupNamedStrDouble(min float64, mNames ...zstr.StrDouble) {
	h.MinValue = min
	for _, si := range mNames {
		class := Class{MaxRange: si.Float, Label: si.Str}
		h.Classes = append(h.Classes, class)
	}
}

func (h *Histogram) SetupNamedRanges(min float64, mNames ...any) {
	h.MinValue = min
	for i := 0; i < len(mNames); i += 2 {
		f, is := mNames[i].(float64)
		if !is {
			n := mNames[i].(int)
			f = float64(n)
		}
		name := mNames[i+1].(string)
		class := Class{MaxRange: f, Label: name}
		h.Classes = append(h.Classes, class)
	}
}

func (h *Histogram) SetupNameClasses(names ...string) {
	for _, n := range names {
		class := Class{Label: n}
		h.Classes = append(h.Classes, class)
	}
}

func (h *Histogram) ClassString() string {
	var out string
	for _, c := range h.Classes {
		str := c.Label
		if c.MaxRange != 0 {
			str = zstr.Concat("/", str, c.MaxRange)
		}
		if out != "" {
			out += " "
		}
		out += fmt.Sprintf("%s:%d", str, c.Count)
	}
	return out
}

func (h *Histogram) FindName(name string) int {
	for i, c := range h.Classes {
		if c.Label == name {
			return i
		}
	}
	return -1
}

func (h *Histogram) AddNameValue(name string) {
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
	if value < h.MinValue {
		h.OutlierBelow++
		h.OutlierBelowSum += value
		return
	}
	if h.AccumilateClasses {
		for i, c := range h.Classes {
			if c.MaxRange == value {
				h.Classes[i].Count++
				return
			}
		}
		c := Class{Count: 1, MaxRange: value}
		h.Classes = append(h.Classes, c)
		return
	}
	for i, c := range h.Classes {
		if value <= c.MaxRange {
			h.Classes[i].Count++
			return
		}
	}
	h.OutlierAbove++
	h.OutlierAboveSum += value
}

func (h *Histogram) TotalCount() int {
	count := h.OutlierAbove + h.OutlierBelow
	for _, c := range h.Classes {
		count += c.Count
	}
	return count
}

func (h *Histogram) Sum() float64 {
	sum := h.OutlierAboveSum + h.OutlierBelowSum
	prev := h.MinValue
	for _, c := range h.Classes {
		diff := c.MaxRange - prev
		sum += diff * float64(c.Count)
		prev = c.MaxRange
	}
	return sum
}

func (h *Histogram) CountAsRatio(c int) float64 {
	total := h.TotalCount()
	return float64(c) / float64(total)
}

func (h *Histogram) CountAsPercent(c int) int {
	return int(100 * h.CountAsRatio(c))
}

func (h *Histogram) MergeIn(in Histogram) {
	found := map[int]bool{}
	for i, c := range h.Classes {
		for j, ic := range in.Classes {
			if ic.MaxRange == c.MaxRange && ic.Label == c.Label {
				h.Classes[i].Count += ic.Count
				found[j] = true
				break
			}
		}
	}
	for j, ic := range in.Classes {
		if !found[j] {
			h.Classes = append(h.Classes, ic)
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

func (h *Histogram) DebugStr() string {
	str := "["
	for i, c := range h.Classes {
		if i != 0 {
			str += " "
		}
		str += fmt.Sprintf("%s:%d,%g", c.Label, c.Count, c.MaxRange)
	}
	str += "]"
	return str
}

func (h *Histogram) NamedClassesSortedByLabel() []Class {
	classes := slices.Clone(h.Classes)
	intSortable := true
	for _, c := range classes {
		_, err := strconv.Atoi(c.Label)
		if err != nil {
			intSortable = false
			break
		}
	}
	if intSortable {
		slices.SortFunc(classes, func(a, b Class) int {
			ia, _ := strconv.Atoi(a.Label)
			ib, _ := strconv.Atoi(b.Label)
			return ia - ib
		})
		return classes
	}
	slices.SortFunc(classes, func(a, b Class) int {
		return strings.Compare(a.Label, b.Label)
	})
	return classes
}

func (h *Histogram) AddClassLabels(labels ...string) {
	for _, label := range labels {
		c := Class{Label: label}
		h.Classes = append(h.Classes, c)
	}
}

func (h *Histogram) HasNames() bool {
	for _, c := range h.Classes {
		if len(c.Label) != 0 {
			return true
		}
	}
	return false
}
