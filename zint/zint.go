package zint

import (
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"strconv"
	"strings"
)

func Int64Abs(i int64) int64 {
	if i >= 0 {
		return i
	}
	return -i
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Maximize(a *int, b int) bool {
	if *a > b {
		return false
	}
	*a = b
	return true
}

func Minimize(a *int, b int) bool {
	if *a < b {
		return false
	}
	*a = b
	return true
}
func Clamp(a, min, max int) int {
	if a < min {
		a = min
	} else if a > max {
		a = max
	}
	return a
}

func Max64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func Maximize64(a *int64, b int64) bool {
	if *a > b {
		return false
	}
	*a = b
	return true
}

func Minimize64(a *int64, b int64) bool {
	if *a < b {
		return false
	}
	*a = b
	return true
}

func IsInSlice64(i int64, slice []int64) bool {
	return IndexInSlice64(i, slice) != -1
}

func IndexInSlice64(n int64, slice []int64) int {
	for i, s := range slice {
		if s == n {
			return i
		}
	}
	return -1
}

func IsInSlice32(n int32, slice []int32) bool {
	for _, s := range slice {
		if s == n {
			return true
		}
	}
	return false
}

func AddToSet64(n int64, slice *[]int64) bool {
	if !IsInSlice64(n, *slice) {
		*slice = append(*slice, n)
		return true
	}
	return false
}

func Join64(ids []int64, sep string) (str string) {
	for i, id := range ids {
		if i != 0 {
			str += sep
		}
		str += strconv.FormatInt(id, 10)
	}
	return
}

func SplitStringTo64(str string, sep string) (ints []int64) {
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

func FromBool(b bool) int {
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

func MapBoolToSlice64(m map[int64]bool) (slice []int64) {
	for n := range m {
		slice = append(slice, n)
	}
	return
}

// func SortedMap(m map[int64]int64, desc bool, each func(k int64, v int64)) {
// 	mapCopy := map[int64]int64{}
// 	for k, v := range m {
// 		mapCopy[k] = v
// 	}
// 	for len(mapCopy) > 0 {
// 		extreme := int64(math.MaxInt64)
// 		if desc {
// 			extreme = math.MinInt64
// 		}
// 		for k, _ := range mapCopy {
// 			if (desc && k > extreme) || (!desc && k < extreme) {
// 				extreme = k
// 			}
// 		}
// 		each(extreme, m[extreme])
// 		delete(mapCopy, extreme)
// 	}
// }

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

func Abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func SetAny(any interface{}, i int64) error {
	switch any.(type) {
	case *int:
		*any.(*int) = int(i)
		fmt.Println("Set Int:", i)
	case *int8:
		*any.(*int8) = int8(i)
	case *int16:
		*any.(*int16) = int16(i)
	case *int32:
		*any.(*int32) = int32(i)
	case *int64:
		*any.(*int64) = int64(i)
	case *uint8:
		*any.(*uint8) = uint8(i)
	case *uint16:
		*any.(*uint16) = uint16(i)
	case *uint32:
		*any.(*uint32) = uint32(i)
	case *uint64:
		*any.(*uint64) = uint64(i)
	default:
		err := errors.New(fmt.Sprint("bad type:", reflect.TypeOf(any)))
		fmt.Println("zint.SetAny err:", err)
		return err
	}
	return nil
}
