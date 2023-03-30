package zmap

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
)

type LockMap[K comparable, V any] struct {
	Map sync.Map
}

// Count() returns the nuber of items in the LockMap.
// Note: It has to use Range() to go through all and count.
func (l *LockMap[K, V]) Count() int {
	var count int
	l.Map.Range(func(k, v any) bool {
		count++
		return true
	})
	return count
}

func (l *LockMap[K, V]) Set(k K, v V) {
	l.Map.Store(k, v)
}

func (l *LockMap[K, V]) Has(k K) bool {
	_, ok := l.Map.Load(k)
	return ok
}

func (l *LockMap[K, V]) Get(k K) (v V, ok bool) {
	a, ok := l.Map.Load(k)
	if ok {
		return a.(V), true
	}
	return
}

func (l *LockMap[K, V]) ForEach(f func(key K, value V) bool) {
	l.Map.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (l *LockMap[K, V]) Remove(k K) {
	l.Map.Delete(k)
}

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

func Keys[K comparable, V any](m map[K]V) []K {
	keys := make([]K, len(m))
	i := 0
	for k := range m {
		keys[i] = k
	}
	return keys
}

func SortedKeys[K comparable, V any](m map[K]V, less func(a, b V) bool) []K {
	keys := Keys(m)
	i := 0
	for k := range m {
		keys[i] = k
	}
	sort.Slice(keys, func(i, j int) bool {
		return less(m[keys[i]], m[keys[j]])
	})
	return keys
}

func SortedValues[K comparable, V any](m map[K]V, less func(a, b V) bool) []V {
	keys := SortedKeys(m, less)
	vals := make([]V, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return vals
}
