package zredis

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

var redisPool *redis.Pool
var rootPath string

func Init(rPool *redis.Pool, path string) {
	if rPool == nil {
		Setup("")
	}
	redisPool = rPool
	rootPath = path
}

func ScanKeys(regex string, handler func(key string) bool) (err error) {
	scursor := "0"
	conn := redisPool.Get()
	defer conn.Close()

	for {
		var skeys []string
		parts, e := redis.Values(conn.Do("SCAN", scursor))
		if e != nil {
			return errors.New("uredis.ScanKeys error: " + e.Error())
		}
		if len(parts) < 2 {
			return
		}
		_, e = redis.Scan(parts, &scursor, &skeys)
		if e != nil {
			return errors.New("uredis.ScanKeys redis.Scan error: " + e.Error())
		}
		if scursor == "0" {
			break
		}
		for _, key := range skeys {
			if !handler(key) {
				return
			}
		}
	}
	return
}

// item is a dummy pointer to struct to unmarshal each item into

func GetRangeToJsonItems(conn redis.Conn, key string, make func() interface{}) (items []interface{}, err error) {
	values, err := redis.Values(conn.Do("ZRANGE", key, 0, -1))
	if err != nil {
		return
	}
	// Get all stories.
	for _, val := range values {
		raw, _ := redis.Bytes(val, nil)
		item := make()
		err = json.Unmarshal(raw, item)
		if err != nil {
			return
		}
		items = append(items, item)
	}
	return
}

// this is a general rotuine to put something in redis with a key, expiry and an interface{} to marshall to json
// It uses redisPool variable for this facebook package
func Put(redisPool *redis.Pool, key string, timeToLive time.Duration, v interface{}) (err error) {
	bjson, err := json.Marshal(v)
	if err != nil {
		return
	}

	conn := redisPool.Get()
	defer conn.Close()

	if rootPath != "" {
		key = rootPath + "/" + key
	}
	err = conn.Send("SET", key, bjson)
	if err != nil {
		return
	}

	if timeToLive != 0 {
		secs := int64(timeToLive / time.Second)
		err = conn.Send("EXPIRE", key, secs)
		if err != nil {
			return
		}
	}

	err = conn.Flush()
	return
}

// this is a general rotuine that gets something from redis with a key, and unmarshals it to a struct
// It uses redisPool variable for this facebook package
func Get(redisPool *redis.Pool, v interface{}, key string) (got bool, err error) {
	conn := redisPool.Get()
	defer conn.Close()

	if rootPath != "" {
		key = rootPath + "/" + key
	}
	// Get and parse result.
	rawCont, err := redis.Bytes(conn.Do("GET", key))
	if err == redis.ErrNil {
		return false, nil
	}
	if err != nil {
		return
	}
	//	zlog.Info("redisget:", string(rawCont))
	err = json.Unmarshal(rawCont, v)
	if err != nil {
		return
	}
	got = true

	return
}

func Delete(redisPool *redis.Pool, key string) (err error) {
	conn := redisPool.Get()
	defer conn.Close()

	if rootPath != "" {
		key = rootPath + "/" + key
	}
	err = conn.Send("DEL", key)

	return
}

// Setup inits redis with address:port. If empty defaults to "localhost:6379"
func Setup(redisServer string) (*redis.Pool, error) {
	if redisServer == "" {
		redisServer = "localhost:6379"
	}
	pool := &redis.Pool{
		MaxIdle:     750,
		MaxActive:   750,
		Wait:        true,
		IdleTimeout: 5 * time.Minute,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", redisServer)
			if err != nil {
				return nil, err
			}
			/*if _, err := c.Do("AUTH", password); err != nil {
				c.Close()
				return nil, err
			}*/
			return c, err
		},
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
	}

	// Try if working.
	conn := pool.Get()
	defer conn.Close()
	_, err := conn.Do("PING")
	if err != nil {
		err = fmt.Errorf("Redis: %s", err)
	}

	return pool, err
}

func GetRotatingIndex(redisPool *redis.Pool, key string, len int, inc bool, timeToLive time.Duration) (n int, rotated bool) {
	Get(redisPool, &n, key)
	if inc {
		i := n + 1
		if i >= len {
			rotated = true
			i = 0
		}
		Put(redisPool, key, timeToLive, i)
	}
	return
}
