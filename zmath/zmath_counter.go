package zmath

import "time"

// TimeWindowCounter is a counter that keeps track of the average count over a time window.
// It is useful for things like bits per second counters.
type TimeWindowCounter struct {
	oldAverage      int64
	totalCount      int64
	lastTotalCount  int64
	lastSampleStart time.Time
	window          time.Duration
	UpdatedFunc     func(int64)
}

func NewTimeWindowCounter(window time.Duration) *TimeWindowCounter {
	return &TimeWindowCounter{
		oldAverage:      0,
		lastSampleStart: time.Now(),
		window:          window,
	}
}

func (b *TimeWindowCounter) SetCount(n int64) {
	if time.Since(b.lastSampleStart) > b.window {
		b.oldAverage = int64(float64(b.totalCount-b.lastTotalCount) / b.window.Seconds())
		b.lastTotalCount = b.totalCount
		if b.UpdatedFunc != nil {
			b.UpdatedFunc(b.oldAverage)
		}
		for { // kinda dangerous
			next := b.lastSampleStart.Add(b.window)
			if time.Since(next) <= 0 {
				break
			}
			b.lastSampleStart = next
		}
	}
	b.totalCount = n
}

// CurrentAverage returns the last FULL average, so it may be a bit old, but it is only updated when a full window has passed.
func (b *TimeWindowCounter) CurrentAverage() int64 {
	return b.oldAverage
}
