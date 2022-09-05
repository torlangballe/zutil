package zcache

import (
	"reflect"
	"sync"
	"time"

	"github.com/torlangballe/zutil/ztimer"
)

// TODO: Allow a redis hookup

type item struct {
	value   interface{}
	touched time.Time
	expiry  time.Duration
}

// Cache is a simple in-memory cached map;
type Cache struct {
	accurateExpiry bool // If accurateExpiry set, items are not touched on get, and Get returns false if expired, even if not flushed out yet. Good for tokens etc that actually have an expiry time
	defaultExpiry  time.Duration
	items          map[string]*item
	lock           sync.Mutex
}

func New(expiry time.Duration, accurateExpiry bool) *Cache {
	c := &Cache{}
	c.items = map[string]*item{}
	c.accurateExpiry = accurateExpiry
	c.defaultExpiry = expiry
	if expiry != 0 {
		ztimer.RepeatIn(60*10, func() bool {
			c.lock.Lock()
			for key, i := range c.items {
				if time.Since(i.touched) > expiry {
					delete(c.items, key)
				}
			}
			c.lock.Unlock()
			return true
		})
	}
	return c
}

func (c *Cache) Put(key string, val interface{}) bool {
	e := c.defaultExpiry
	return c.PutWithExpiry(key, val, e)
}

func (c *Cache) PutWithExpiry(key string, val interface{}, expiry time.Duration) bool {
	c.lock.Lock()
	i, _ := c.items[key]
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

func (c *Cache) Get(toPtr interface{}, key string) (got bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	i, _ := c.items[key]
	if i == nil {
		return false
	}
	if c.accurateExpiry && i.expiry != 0 {
		if time.Since(i.touched) >= i.expiry {
			delete(c.items, key)
			return false
		}
	}
	if !c.accurateExpiry {
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
	delete(c.items, key)
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
