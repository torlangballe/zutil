package zcache

import (
	"reflect"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// TODO: rename fixedExpiry to NonExcedableExpiry?
// TODO: Allow a redis hookup?

type Releaser interface {
	Release()
}

type item struct {
	value   any
	touched time.Time
	expiry  time.Duration
}

// Cache is a simple in-memory cached map. If it's value conforms to Releaser (above), Release() is called on it before removing from cache.
type Cache struct {
	fixedExpiry   bool // If fixedExpiry set, items are not touched on get, and Get returns false if expired, even if not flushed out yet. Good for tokens etc that actually have an expiry time
	defaultExpiry time.Duration
	hasExpiries   bool
	items         map[string]*item
	lock          sync.Mutex
}

// New creates a new a cache with no expiry
func New() *Cache {
	return NewWithExpiry(0, false)
}

func NewWithExpiry(expirySecs float64, fixed bool) *Cache {
	c := &Cache{}
	c.items = map[string]*item{}
	c.defaultExpiry = ztime.SecondsDur(expirySecs)
	c.fixedExpiry = fixed
	ztimer.Repeat(60*10, func() bool {
		if c.hasExpiries {
			c.purge()
			return true
		}
		return true
	})
	return c
}

func (c *Cache) Count() int {
	var count int
	c.lock.Lock()
	for key, i := range c.items {
		if time.Since(i.touched) > i.expiry {
			delete(c.items, key)
		} else {
			count++
		}
	}
	c.lock.Unlock()
	return count
}

func (c *Cache) Put(key string, val interface{}) bool {
	return c.PutWithExpiry(key, val, c.defaultExpiry)
}

func (c *Cache) PutWithExpiry(key string, val interface{}, expiry time.Duration) bool {
	zlog.Assert(expiry != 0 || !c.fixedExpiry)
	if expiry != 0 {
		c.hasExpiries = true
	}
	c.lock.Lock()
	i := c.items[key]
	if i == nil {
		i = &item{}
		c.items[key] = i
	}
	i.touched = time.Now()
	i.value = val
	i.expiry = expiry
	c.lock.Unlock()
	return false
}

func (c *Cache) purge() {
	c.lock.Lock()
	for key, i := range c.items {
		if time.Since(i.touched) >= i.expiry {
			c.release(i)
			delete(c.items, key)
		}
	}
	c.hasExpiries = false
	c.lock.Unlock()
}

func (c *Cache) Get(toPtr interface{}, key string) (got bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	i := c.items[key]
	if i == nil {
		return false
	}
	if c.fixedExpiry {
		if time.Since(i.touched) >= i.expiry {
			delete(c.items, key)
			return false
		}
	} else {
		i.touched = time.Now()
	}
	a := reflect.ValueOf(toPtr).Elem()
	if i.value == nil {
		a.Set(reflect.Zero(a.Type()))
	} else {
		v := reflect.ValueOf(i.value)
		a.Set(v)
	}
	return true
}

func (c *Cache) Remove(key string) {
	c.lock.Lock()
	i, got := c.items[key]
	if got {
		c.release(i)
		delete(c.items, key)
	}
	c.lock.Unlock()
}

func (c *Cache) ForAll(f func(key string, value interface{}) bool) {
	c.lock.Lock()
	for k, v := range c.items {
		if !f(k, v) {
			break
		}
	}
	c.lock.Unlock()
}

func (c *Cache) release(i *item) {
	c.lock.Lock()
	if i.value != nil {
		releaser, _ := i.value.(Releaser)
		if releaser != nil {
			releaser.Release()
		}
	}
	c.lock.Unlock()
}
