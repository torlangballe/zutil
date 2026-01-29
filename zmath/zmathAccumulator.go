package zmath

type Accumulator struct {
	Count int
	Sum   float64
	Min   float64
	Max   float64
}

func (a *Accumulator) Add(value float64) {
	a.Count++
	a.Sum += value
	if a.Count == 1 || value < a.Min {
		a.Min = value
	}
	if a.Count == 1 || value > a.Max {
		a.Max = value
	}
}

func (a *Accumulator) Average() float64 {
	if a.Count == 0 {
		return 0
	}
	return a.Sum / float64(a.Count)
}
