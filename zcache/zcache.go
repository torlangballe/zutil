package zcache

import (
	"reflect"

	"github.com/patrickmn/go-cache"
	"github.com/torlangballe/zutil/ztime"
)

// TODO: Allow a redis hookup

var DefaultExpiration = 3600.0

const NoExpiration = -1.0

func New(ttlSecs float64) *Cache {
	if ttlSecs == 0 {
		ttlSecs = 3600 * 24
	}
	c := new(Cache)
	c.cache = cache.New(ztime.SecondsDur(ttlSecs), ztime.SecondsDur(ttlSecs*2))
	return c
}

func (c *Cache) Put(key string, val interface{}) error {
	c.cache.Set(key, val, cache.DefaultExpiration)
	return nil
}

func (c *Cache) PutTTL(key string, val interface{}, ttlSecs float64) error {
	if ttlSecs == NoExpiration {
		ttlSecs = ztime.DurSeconds(cache.NoExpiration)
	} else {
		ttlSecs = ztime.DurSeconds(cache.DefaultExpiration)
	}
	c.cache.Set(key, val, ztime.SecondsDur(ttlSecs))
	return nil
}

func (c *Cache) Get(key string) (val interface{}, got bool) {
	val, got = c.cache.Get(key)
	return
}

func (c *Cache) GetTo(toPtr interface{}, key string) (got bool) {
	val, got := c.cache.Get(key)
	if got {
		a := reflect.ValueOf(toPtr).Elem()
		v := reflect.ValueOf(val)
		a.Set(v)
	}
	return
}

func (c *Cache) Delete(key string) {
	c.cache.Delete(key)
}

type Cache struct {
	cache *cache.Cache
}
