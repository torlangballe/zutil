package zcache

//	"github.com/torlangballe/zutil/zmap"

import (
	"time"

	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type ExpiringMap[K comparable, V any] struct {
	lockedMap zmap.LockMap[K, struct {
		value   V
		touched time.Time
	}]
	secsToLive float64
}

func NewExpiringMap[K comparable, V any](secsToLive float64) *ExpiringMap[K, V] {
	m := &ExpiringMap[K, V]{}
	m.secsToLive = secsToLive
	ztimer.RepeatForever(secsToLive/3, func() {
		m.lockedMap.ForEach(func(k K, v struct {
			value   V
			touched time.Time
		}) bool {
			if ztime.Since(v.touched) > secsToLive {
				m.lockedMap.Remove(k)
			}
			return true
		})
	})
	return m
}

func (m *ExpiringMap[K, V]) Set(k K, v V) {
	m.lockedMap.Set(k, struct {
		value   V
		touched time.Time
	}{v, time.Now()})
}

func (m *ExpiringMap[K, V]) Get(k K) (V, bool) {
	val, ok := m.lockedMap.Get(k)
	if ok {
		if ztime.Since(val.touched) > m.secsToLive {
			m.lockedMap.Remove(k)
		} else {
			val.touched = time.Now()
			m.lockedMap.Set(k, val)
		}
		return val.value, true
	}
	return val.value, false
}

func (m *ExpiringMap[K, V]) Remove(k K) {
	m.lockedMap.Remove(k)
}