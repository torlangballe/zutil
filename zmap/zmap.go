package zmap

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
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

// If k exists in l, GetSet returns it and true, otherwise k is set with def and def, false is returned
func (l *LockMap[K, V]) GetSet(k K, def V) (V, bool) {
	a, loaded := l.Map.LoadOrStore(k, def)
	return a.(V), loaded
}

func (l *LockMap[K, V]) Get(k K) (V, bool) {
	a, ok := l.Map.Load(k)
	if ok {
		return a.(V), true
	}
	var v V
	return v, false
}

func (l *LockMap[K, V]) Index(k K) V {
	v, _ := l.Get(k)
	return v
}

func (l *LockMap[K, V]) Pop(k K) (v V, ok bool) {
	a, ok := l.Map.Load(k)
	if ok {
		l.Map.Delete(k)
		return a.(V), true
	}
	return
}

func (l *LockMap[K, V]) ForEach(f func(key K, value V) bool) {
	l.Map.Range(func(k, v any) bool {
		return f(k.(K), v.(V))
	})
}

func (l *LockMap[K, V]) ForAll(f func(key K, value V)) {
	l.Map.Range(func(k, v any) bool {
		f(k.(K), v.(V))
		return true
	})
}

func (l *LockMap[K, V]) AnyKey() K {
	var key K
	l.ForEach(func(k K, v V) bool {
		key = k
		return false
	})
	return key
}

func (l *LockMap[K, V]) Remove(k K) {
	l.Map.Delete(k)
}

func (l *LockMap[K, V]) RemoveAll() {
	l.Map = sync.Map{}
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
		i++
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

func KeySortedValues[K comparable, V any](m map[K]V, less func(a, b V) bool) []V {
	keys := SortedKeys(m, less)
	vals := make([]V, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return vals
}

func AllValues[K comparable, V any](m map[K]V) []V {
	keys := Keys(m)
	out := make([]V, len(keys))
	for i, key := range keys {
		out[i] = m[key]
	}
	return out
}

// For each /-separated name in m, GetValueInRecursiveMap, if not at end or a map, will recursivly gets next part in that map.
func GetValueInRecursiveMap(m map[string]any, slashPath string) (any, error) {
	for i, part := range strings.Split(slashPath, "/") {
		v, got := m[part]
		if !got {
			return nil, errors.New("part not found")
		}
		if i == len(slashPath)-1 { // if no more path left, return the map as value
			return v, nil
		}
		m2 := v.(map[string]any)
		if m2 != nil {
			m = m2
			continue
		}
		return v, fmt.Errorf("not at end, but not a map: %d %s %+v", i, part, v)
	}
	return nil, nil // can't get here
}

// EmptyOf returns an empty list like m.
// Usefull since maps are shared and you want to avoid clearing a shared map.
func EmptyOf[K comparable, V any](m map[K]V) map[K]V {
	return map[K]V{}
}
