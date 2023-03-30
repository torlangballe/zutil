package ztimer

import (
	"math"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

// create a RateLimiter to only do a function every n seconds since last time it was done for a given id.
// It bases this on the time since the previous function was **started**.
// If maxSecs is non-zero, it increases frequency logarithmically StepsToMax times til it is maxSecs,
// setting back to start if DoBackoff returns true.
type RateLimiter struct {
	maxSecs   float64 // If non-zero, it increases time until next Do up til maxFrequencySecs, x^1.n=y
	startSecs float64
	multiply  float64
	last      time.Time
	freqSecs  float64 // the minimum time until Do function should run again
	executing bool

	StepsToMax int
}

type RateLimiters struct {
	cache       map[string]*RateLimiter
	lock        sync.Mutex
	defaultSecs float64
	StepsToMax  int
}

func NewRateLimiter(secs, maxSecs float64) *RateLimiter {
	r := &RateLimiter{}
	r.startSecs = secs
	r.freqSecs = secs
	r.maxSecs = maxSecs
	r.StepsToMax = 10
	return r
}

func (r *RateLimiter) Do(do func()) {
	if r.executing {
		return
	}
	ready := (ztime.Since(r.last) > r.freqSecs)
	if ready {
		r.last = time.Now()
		r.executing = true
		do()
		r.executing = false
	}
}

func (r *RateLimiter) DoBackoff(do func() bool) {
	if r.executing {
		return
	}
	zlog.Assert(r.maxSecs != 0)
	if r.multiply == 0 {
		r.multiply = math.Pow(r.maxSecs, 1/float64(r.StepsToMax)) / math.Pow(r.freqSecs, 1/float64(r.StepsToMax))
	} else {
		if r.freqSecs+0.000001 < r.maxSecs {
			// zlog.Info("DoBackoff:", r.multiply, r.freqSecs, r.startSecs)
			r.freqSecs *= r.multiply
		}
	}
	ready := (ztime.Since(r.last) > r.freqSecs)
	if ready {
		r.last = time.Now()
	}
	if ready {
		r.executing = true
		if do() {
			// zlog.Info("rate limiter restart")
			r.multiply = 0
			r.freqSecs = r.startSecs
		}
		r.executing = false
	}
}

// RateLimiters ****************************************************************

func NewRateLimiters(secs float64) *RateLimiters {
	r := &RateLimiters{}
	r.cache = map[string]*RateLimiter{}
	r.defaultSecs = secs
	r.StepsToMax = 10
	RepeatForever(62, func() {
		r.lock.Lock()
		for id, rl := range r.cache {
			max := math.Max(rl.startSecs, rl.maxSecs)
			if !rl.last.IsZero() && ztime.Since(rl.last) > max+50 {
				delete(r.cache, id)
			}
		}
		r.lock.Unlock()
	})
	return r
}

// If no rate limiter with id exists, one is added with id and secs.
// if secs == 0, r.defaultSecs is used
func (r *RateLimiters) Do(id string, secs float64, do func()) {
	r.lock.Lock()
	rc := r.cache[id]
	r.lock.Unlock() // unlock before r.Add(), it locks
	if rc == nil {
		rc = r.Add(id, secs, 0)
	}
	rc.Do(do)
}

func (r *RateLimiters) DoBackoff(id string, secs, maxSecs float64, do func() bool) {
	r.lock.Lock()
	rc := r.cache[id]
	r.lock.Unlock() // unlock before r.Add(), it locks
	if rc == nil {
		rc = r.Add(id, secs, maxSecs)
	}
	rc.DoBackoff(do)
}

// Adds a rate limiter with id/secs. If secs == 0, r.defaultSecs is used
func (r *RateLimiters) Add(id string, secs, maxSecs float64) *RateLimiter {
	if secs <= 0 {
		secs = r.defaultSecs
		if secs == 0 {
			panic("no default secs")
		}
	}
	rc := NewRateLimiter(secs, maxSecs)
	rc.StepsToMax = r.StepsToMax
	r.lock.Lock()
	r.cache[id] = rc
	r.lock.Unlock()
	return rc
}
