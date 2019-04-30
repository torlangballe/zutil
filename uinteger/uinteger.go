package uinteger

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"
)

func IsInt64InSlice(i int64, slice []int64) bool {
	return IndexOfInt64InSlice(i, slice) != -1
}

func IndexOfInt64InSlice(n int64, slice []int64) int {
	for i, s := range slice {
		if s == n {
			return i
		}
	}
	return -1
}

func IsInt32InSlice(n int32, slice []int32) bool {
	for _, s := range slice {
		if s == n {
			return true
		}
	}
	return false
}

func AddIntInt64ToSet(n int64, slice *[]int64) bool {
	if !IsInt64InSlice(n, *slice) {
		*slice = append(*slice, n)
		return true
	}
	return false
}

func Join(ids []int64, sep string) (str string) {
	for i, id := range ids {
		if i != 0 {
			str += sep
		}
		str += strconv.FormatInt(id, 10)
	}
	return
}

func SplitString(str string, sep string) (ints []int64) {
	if len(str) == 0 {
		return
	}
	for _, s := range strings.Split(str, sep) {
		s = strings.TrimSpace(s)
		i, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			ints = append(ints, i)
		}
	}
	return ints
}

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func MapBoolToSlice(m map[int]bool) (slice []int) {
	for n := range m {
		slice = append(slice, n)
	}
	return
}

func MapInt64BoolToSlice(m map[int64]bool) (slice []int64) {
	for n := range m {
		slice = append(slice, n)
	}
	return
}

func SortedMap(m map[int64]int64, desc bool, each func(k int64, v int64)) {
	mapCopy := map[int64]int64{}
	for k, v := range m {
		mapCopy[k] = v
	}
	for len(mapCopy) > 0 {
		extreme := int64(math.MaxInt64)
		if desc {
			extreme = math.MinInt64
		}
		for k, _ := range mapCopy {
			if (desc && k > extreme) || (!desc && k < extreme) {
				extreme = k
			}
		}
		each(extreme, m[extreme])
		delete(mapCopy, extreme)
	}
}

func HashTo32(str string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(str))
	return h.Sum32()
}

func HashTo64(str string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(str))
	return h.Sum64()
}

func AbsInt(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
