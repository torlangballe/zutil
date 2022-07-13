package ztimer

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/ztime"
)

// create a RateLimiter to only do a function every n seconds since last time it was done for a given id.
// It bases this on the time since the previous function was **started**.
type RateLimiter struct {
	last          time.Time
	frequencySecs float64
	executing     bool
}

func NewRateLimiter(secs float64) *RateLimiter {
	r := &RateLimiter{}
	r.frequencySecs = secs
	return r
}

// Do runs the *do* function if secs -1) has passed since last time it was done for that id,
// and it is not still execting previois do()
func (r *RateLimiter) Do(do func()) {
	if r.executing {
		return
	}
	ready := (ztime.Since(r.last) > r.frequencySecs)
	if ready {
		r.last = time.Now()
	}
	if ready {
		r.executing = true
		do()
		r.executing = false
	}
}

type RateLimiters struct {
	cache       map[int64]*RateLimiter
	lock        sync.Mutex
	defaultSecs float64
}

func NewRateLimiters(secs float64) *RateLimiters {
	r := &RateLimiters{}
	r.cache = map[int64]*RateLimiter{}
	r.defaultSecs = secs
	return r
}

// If no rate limiter with id exists, one is added with id and secs.
// if secs == 0, r.defaultSecs is used
func (r *RateLimiters) Do(id int64, secs float64, do func()) {
	r.lock.Lock()
	rc := r.cache[id]
	r.lock.Unlock() // unlock before r.Add(), it locks
	if rc == nil {
		rc = r.Add(id, secs)
	}
	rc.Do(do)
}

// Adds a rate limiter with id/secs. If secs == 0, r.defaultSecs is used
func (r *RateLimiters) Add(id int64, secs float64) *RateLimiter {
	if secs <= 0 {
		secs = r.defaultSecs
		if secs == 0 {
			panic("no default secs")
		}
	}
	rc := NewRateLimiter(secs)
	r.lock.Lock()
	r.cache[id] = rc
	r.lock.Unlock()
	return rc
}
