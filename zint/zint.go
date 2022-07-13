package zint

import (
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

type Slice []int

var dummy int

// SizeOfInt is the size of an int type
var SizeOfInt = int(unsafe.Sizeof(dummy))

func Abs64(i int64) int64 {
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

func MinMax(a, b int) (min, max int) {
	if a < b {
		return a, b
	}
	return b, a
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

func RemoveFromSet64(n int64, set *[]int64) bool {
	for i, num := range *set {
		if num == n {
			*set = append((*set)[:i], (*set)[i+1:]...)
			return true
		}
	}
	return false
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
	return StringsTo64(strings.Split(str, sep))
}

func StringsTo64(snums []string) (ints []int64) {
	for _, s := range snums {
		s = strings.TrimSpace(s)
		i, err := strconv.ParseInt(s, 10, 64)
		if err == nil {
			ints = append(ints, i)
		}
	}
	return
}

func ToStrings64(ints []int64) (s []string) {
	for _, n := range ints {
		s = append(s, strconv.FormatInt(n, 10))
	}
	return
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

func HashTo32(str string) int32 {
	h := fnv.New32a()
	h.Write([]byte(str))
	n := int32(h.Sum32() >> 1)
	if n < 0 {
		panic(n)
	}
	return n
}

func HashTo64(str string) int64 {
	h := fnv.New64a()
	h.Write([]byte(str))
	n := int64(h.Sum64() >> 1)
	if n < 0 {
		panic(n)
	}
	return n
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
		err := errors.New(fmt.Sprint("bad type:", reflect.TypeOf(any))) // don't use zlog, will be import cycle
		fmt.Println("zint.SetAny err:", err)
		return err
	}
	return nil
}

func GetAny(i interface{}) (int64, error) {
	if i == nil {
		return 0, errors.New("is nil")
	}
	switch i.(type) {
	case bool:
		if i.(bool) {
			return 1, nil
		}
		return 0, nil
	case int:
		return int64(i.(int)), nil
	case int8:
		return int64(i.(int8)), nil
	case int16:
		return int64(i.(int16)), nil
	case int32:
		return int64(i.(int32)), nil
	case int64:
		return int64(i.(int64)), nil
	case uint:
		return int64(i.(uint)), nil
	case uint8:
		return int64(i.(uint8)), nil
	case uint16:
		return int64(i.(uint16)), nil
	case uint32:
		return int64(i.(uint32)), nil
	case uint64:
		return int64(i.(uint64)), nil
	case float32:
		return int64(i.(float32)), nil
	case float64:
		return int64(i.(float64)), nil
	case string:
		n, err := strconv.ParseInt(i.(string), 10, 64)
		return n, err
	}
	val := reflect.ValueOf(i)
	switch val.Kind() {
	case reflect.Bool:
		if val.Bool() {
			return 1, nil
		}
		return 0, nil
	case reflect.Int:
		return int64(val.Int()), nil
	case reflect.Int8:
		return int64(val.Int()), nil
	case reflect.Int16:
		return int64(val.Int()), nil
	case reflect.Int32:
		return int64(val.Int()), nil
	case reflect.Int64:
		return int64(val.Int()), nil
	case reflect.Uint:
		return int64(val.Int()), nil
	case reflect.Uint8:
		return int64(val.Int()), nil
	case reflect.Uint16:
		return int64(val.Int()), nil
	case reflect.Uint32:
		return int64(val.Int()), nil
	case reflect.Uint64:
		return int64(val.Int()), nil
	case reflect.Float32:
		return int64(val.Float()), nil
	case reflect.Float64:
		return int64(val.Float()), nil
	case reflect.String:
		n, err := strconv.ParseInt(val.String(), 10, 64)
		return n, err
	}
	return 0, fmt.Errorf("bad kind %v", reflect.ValueOf(i).Kind())

}

// GetItem makes Slice worth with MenuView MenuItems interface
func (s Slice) GetItem(i int) (id, name string, value interface{}) {
	if i >= len(s) {
		return "", "", nil
	}
	n := s[i]
	str := strconv.Itoa(n)
	return str, str, n
}

func (s Slice) Count() int {
	return len(s)
}

func (s Slice) Minimum() int {
	var min int
	for i, n := range s {
		if i == 0 || min > n {
			min = n
		}
	}
	return min
}
