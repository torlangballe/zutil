package zmap

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

func GetAnyKeyAsString(m interface{}) string {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return ""
	}
	str, _ := keys[0].Interface().(string)
	return str
}

func GetAnyValue(getPtr interface{}, m interface{}) error {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return errors.New("no items")
	}
	v := mval.MapIndex(keys[0])
	reflect.ValueOf(getPtr).Elem().Set(v)
	return nil
}

func GetKeysAsStrings(m interface{}) (keys []string) {
	mval := reflect.ValueOf(m)
	mkeys := mval.MapKeys()
	for i := 0; i < len(mkeys); i++ {
		k := fmt.Sprint(mkeys[i].Interface())
		keys = append(keys, k)
	}
	return
}

type LockMap[K comparable, V any] struct {
	sync.Map
}

// Count() returns the nuber of items in the LockMap.
// Note: It has to use Range() to go through all and count.
func (l *LockMap[K, V]) Count() int {
	var count int
	l.Range(func(k, v any) bool {
		count++
		return true
	})
	return count
}

func (l *LockMap[K, V]) Set(k K, v V) {
	l.Store(k, v)
}

func (l *LockMap[K, V]) Get(k K) (v V, ok bool) {
	a, ok := l.Load(k)
	if ok {
		return a.(V), true
	}
	return
}

func (l *LockMap[K, V]) ForEach(f func(key K, value V) bool) {
	l.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (l *LockMap[K, V]) Remove(k K) {
	l.Delete(k)
}
