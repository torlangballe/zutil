package zmap

import (
	"cmp"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
)

type LockMap[K comparable, V any] struct {
	Map sync.Map
}

// Count() returns the number of items in the LockMap.
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

// If k exists in l, GetSet returns it and true, otherwise k is set with
// f and def, false is returned
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

func GetAnyKeyAsString(m any) string {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return ""
	}
	str, _ := keys[0].Interface().(string)
	return str
}

func GetAnyValue(getPtr any, m any) error {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return errors.New("no items")
	}
	v := mval.MapIndex(keys[0])
	reflect.ValueOf(getPtr).Elem().Set(v)
	return nil
}

func KeysAsStrings[K comparable, V any](m map[K]V) []string {
	out := make([]string, 0, len(m))
	for _, k := range Keys(m) {
		out = append(out, fmt.Sprint(k))
	}
	return out
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

// func Sorted[S ~[]E, E cmp.Ordered](x S) []E {
// 	n := make([]E, len(x))
// 	copy(n, x)
// 	slices.Sort(n)
// 	return n
// }

func FuncSortedKeys[K comparable, V any](m map[K]V, less func(a, b V) bool) []K {
	keys := Keys(m)
	sort.Slice(keys, func(i, j int) bool {
		return less(m[keys[i]], m[keys[j]])
	})
	return keys
}

func KeySortedValues[K cmp.Ordered, V any](m map[K]V) []V {
	keys := Keys(m)
	slices.Sort(keys)
	out := make([]V, len(keys))
	for i, k := range keys {
		out[i] = m[k]
	}
	return out
}

func SortedKeyValues[K comparable, V any](m map[K]V, less func(a, b V) bool) ([]K, []V) {
	keys := FuncSortedKeys(m, less)
	vals := make([]V, len(keys))
	for i, k := range keys {
		vals[i] = m[k]
	}
	return keys, vals
}

func AllValues[K comparable, V any](m map[K]V) []V {
	keys := Keys(m)
	out := make([]V, len(keys))
	for i, key := range keys {
		out[i] = m[key]
	}
	return out
}

// Empty deletes all keys from map. Usefull if you want to keep the variable referencing the map but clear it.
func Empty[K comparable, V any](m map[K]V) {
	for k := range m {
		delete(m, k)
	}
}

func KeyForValue[K comparable, V comparable](m map[K]V, value V) K {
	for k, v := range m {
		if v == value {
			return k
		}
	}
	var empty K
	return empty
}

// For each /-separated name in m, GetValueInRecursiveMap, if not at end or a map, will recursively gets next part in that map.
func GetValueInRecursiveMap(m map[string]any, slashPath string) (any, error) {
	parts := strings.Split(slashPath, "/")
	for i, part := range parts {
		v, got := m[part]
		if !got {
			return nil, errors.New("part not found")
		}
		if i == len(parts)-1 { // if no more path left, return the map as value
			return v, nil
		}
		m2, _ := v.(map[string]any)
		// fmt.Println("Here", m2 != nil, i, len(slashPath), reflect.TypeOf(v))
		if m2 != nil {
			m = m2
			continue
		}
		return v, fmt.Errorf("get: not at end, but not a map: %d %s %+v map:%+v", i, part, v, m)
	}
	return nil, nil // can't get here
}

func SetValueInRecursiveMap(m map[string]any, slashPath string, val any) error {
	parts := strings.Split(slashPath, "/")
	for i, part := range parts {
		if i == len(parts)-1 {
			m[part] = val
			return nil
		}
		m2, got := m[part].(map[string]any)
		if !got {
			m2 = map[string]any{}
			m[part] = m2
		}
		m = m2
	}
	return nil
}

// EmptyOf returns an empty list like m.
// Usefull since maps are shared and you want to avoid clearing a shared map.
func EmptyOf[K comparable, V any](m map[K]V) map[K]V {
	return map[K]V{}
}

func Copy[K comparable, V any](m map[K]V) map[K]V {
	n := EmptyOf(m)
	for k, v := range m {
		n[k] = v
	}
	return n
}

func Filter[K comparable, V any](m map[K]V, keep func(k K, v V) bool) map[K]V {
	n := map[K]V{}
	for k, v := range m {
		if keep(k, v) {
			n[k] = v
		}
	}
	return n
}

func MapToSlice[K comparable, V any, O any](m map[K]V, tranform func(k K, v V) O) []O {
	o := make([]O, len(m))
	for k, v := range m {
		o = append(o, tranform(k, v))
	}
	return o
}
