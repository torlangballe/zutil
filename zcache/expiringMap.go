package zcache

// ExpiringMap is a thread-safe map which periodically sweeps through and removes items untouched
// longer than secsToLive. If SetStorage is called with a path, it loads, and periodically stores itself.
// Call FlushToStorage() to save any latest entries to storage.

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
	secsToLive  float64
	storagePath string
	changed     bool
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
				// zlog.Info("Expiring: Purge!", k)
				m.lockedMap.Remove(k)
			}
			return true
		})
	})
	return m
}

func (m *ExpiringMap[K, V]) Set(k K, v V) {
	m.changed = true
	m.lockedMap.Set(k, struct {
		value   V
		touched time.Time
	}{v, time.Now()})
}

func (m *ExpiringMap[K, V]) SetForever(k K, v V) {
	m.changed = true
	m.lockedMap.Set(k, struct {
		value   V
		touched time.Time
	}{v, ztime.BigTime})
}

func (m *ExpiringMap[K, V]) get(k K, touch bool) (V, bool) {
	val, ok := m.lockedMap.Get(k)
	if ok {
		// zlog.Info("Peek:", k, ztime.Since(val.touched), m.secsToLive)
		if ztime.Since(val.touched) > m.secsToLive {
			m.lockedMap.Remove(k)
			return val.value, false
		} else {
			if touch {
				val.touched = time.Now()
				m.lockedMap.Set(k, val)
			}
		}
		return val.value, true
	}
	return val.value, false
}

func (m *ExpiringMap[K, V]) Get(k K) (V, bool) {
	return m.get(k, true)
}

func (m *ExpiringMap[K, V]) Peek(k K) (V, bool) {
	return m.get(k, false)
}

func (m *ExpiringMap[K, V]) Remove(k K) {
	m.changed = true
	m.lockedMap.Remove(k)
}

func (m *ExpiringMap[K, V]) ForEach(f func(key K, value V) bool) {
	m.lockedMap.ForEach(func(key K, val struct {
		value   V
		touched time.Time
	}) bool {
		return f(key, val.value)
	})
}

func (m *ExpiringMap[K, V]) ForAll(f func(key K, value V)) {
	m.ForEach(func(key K, value V) bool {
		f(key, value)
		return true
	})
}
