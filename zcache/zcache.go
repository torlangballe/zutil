package zcache

import (
	"fmt"
	"github.com/torlangballe/zutil/ztime"

	"github.com/patrickmn/go-cache"
)

// TODO: Allow a redis hookup

const DefaultExpiration = 0.0
const NoExpiration = -1.0

func New(ttlSecs float64) *Cache {
	c := new(Cache)
	c.cache = cache.New(ztime.Second(ttlSecs), ztime.Second(ttlSecs*2))
	return c
}

func (c *Cache) Store(key interface{}, val interface{}) {
	c.StoreWithTTL(key, val, DefaultExpiration)
}

func (c *Cache) StoreWithTTL(key interface{}, val interface{}, ttlSecs float64) {
	if ttlSecs == DefaultExpiration {
		ttlSecs = ztime.Seconds(cache.DefaultExpiration)
	} else if ttlSecs == NoExpiration {
		ttlSecs = ztime.Seconds(cache.NoExpiration)
	}
	skey := fmt.Sprintf("%v", key)
	c.cache.Set(skey, val, ztime.Second(ttlSecs))
}

func (c *Cache) Load(key interface{}) (val interface{}, got bool) {
	skey := fmt.Sprintf("%v", key)
	val, got = c.cache.Get(skey)
	return
}

func (c *Cache) Delete(key interface{}) {
	skey := fmt.Sprintf("%v", key)
	c.cache.Delete(skey)
}

type Cache struct {
	cache *cache.Cache
}
