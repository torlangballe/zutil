package zmath

import "github.com/torlangballe/zutil/zwords"

type Range[N int | int64 | float64] struct {
	Valid bool `json:",omitempty"`
	Min   N    `json:",omitempty"`
	Max   N    `json:",omitempty"`
}

type RangeF64 = Range[float64]

func (r *Range[N]) Set(min, max N) {
	r.Min = min
	r.Max = max
	r.Valid = true
}

func MakeRange[N int | int64 | float64](min, max N) Range[N] {
	return Range[N]{Valid: true, Min: min, Max: max}
}

func (r Range[N]) Length() N {
	return r.Max - r.Min
}

func (r Range[N]) Added(n N) Range[N] {
	if !r.Valid {
		r.Valid = true
		r.Min = n
		r.Max = n
		return r
	}
	r.Min = min(r.Min, n)
	r.Max = max(r.Max, n)
	return r
}

func (r *Range[N]) Add(n N) {
	*r = r.Added(n)
}

func (r *Range[N]) NiceString(digits int) string {
	if !r.Valid {
		return "invalid"
	}
	min := zwords.NiceFloat(float64(r.Min), digits)
	max := zwords.NiceFloat(float64(r.Max), digits)
	return min + " - " + max
}

func (r *Range[N]) T(t float64) N {
	return r.Min + N(float64(r.Length())*t)
}

// Contracted reduces the range to only just include n at min or max, depending on which is nearer.
// An n outside does nothing.
func (r Range[N]) Contracted(n N) Range[N] {
	if !r.Valid {
		return r
	}
	dMax := r.Max - n
	dMin := n - r.Min
	if dMax > 0 && dMax < dMin {
		r.Max = n
		return r
	}
	if dMin > 0 {
		r.Min = n
		return r
	}
	return r
}

func (r *Range[N]) Contract(n N) {
	*r = r.Contracted(n)
}

// ContractedExcludingInt is like Contracted, but reduces to, but doesn't include nearest int64
func (r Range[N]) ContractedExcludingInt(n int64) Range[N] {
	if !r.Valid {
		return r
	}
	dMax := int64(r.Max) - n
	dMin := n - int64(r.Min)
	if dMax > 0 && dMax < dMin {
		r.Max = N(n) - 1
		return r
	}
	if dMin > 0 {
		r.Min = N(n) + 1
		return r
	}
	return r
}

func (r *Range[N]) ContractExcludingInt(n int64) {
	*r = r.ContractedExcludingInt(n)
}

func (r Range[N]) Clamped(n N) N {
	if !r.Valid {
		return n
	}
	if n < r.Min {
		return r.Min
	}
	if n > r.Max {
		return r.Max
	}
	return n
}

func GetRangeMins[N int | int64 | float64](rs []Range[N]) []N {
	var all []N
	for _, r := range rs {
		if r.Valid {
			all = append(all, r.Min)
		}
	}
	return all
}

func GetRangeMaxes[N int | int64 | float64](rs []Range[N]) []N {
	var all []N
	for _, r := range rs {
		if r.Valid {
			all = append(all, r.Max)
		}
	}
	return all
}

func (r Range[N]) Overlaps(o Range[N]) bool {
	if !r.Valid || !o.Valid {
		return false
	}
	return r.Max >= o.Min && r.Min <= o.Max
}
