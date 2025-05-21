package zint

import (
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"unsafe"
)

type Slice []int

type ID64Setter interface {
	SetID64(id int64)
}

type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

var dummy int

// SizeOfInt is the size of an int type
var SizeOfInt = int(unsafe.Sizeof(dummy))

const Undefined = math.MaxInt

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

func RemoveFromSet64(n int64, set *[]int64) bool {
	for i, num := range *set {
		if num == n {
			*set = append((*set)[:i], (*set)[i+1:]...)
			return true
		}
	}
	return false
}

func SubtractedSets64(set []int64, subtract []int64) []int64 {
	var ns []int64
	for _, n := range set {
		if !Slice64Contains(subtract, n) {
			ns = append(ns, n)
		}
	}
	return ns
}

func Intersection64(a, b []int64) []int64 {
	var ns []int64
	for _, n := range a {
		if Slice64Contains(b, n) {
			ns = append(ns, n)
		}
	}
	return ns
}

func Slice64Contains(slice []int64, n int64) bool {
	for _, s := range slice {
		if s == n {
			return true
		}
	}
	return false
}

func AddToSet[N Integer](n N, slice *[]N) bool {
	if !slices.Contains(*slice, n) {
		*slice = append(*slice, n)
		return true
	}
	return false
}

func AddToSet64(n int64, slice *[]int64) bool {
	if !Slice64Contains(*slice, n) {
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

func SetAny(numPtr any, i int64) error {
	switch numPtr.(type) {
	case *int:
		*numPtr.(*int) = int(i)
	case *int8:
		*numPtr.(*int8) = int8(i)
	case *int16:
		*numPtr.(*int16) = int16(i)
	case *int32:
		*numPtr.(*int32) = int32(i)
	case *int64:
		*numPtr.(*int64) = int64(i)
	case *uint8:
		*numPtr.(*uint8) = uint8(i)
	case *uint16:
		*numPtr.(*uint16) = uint16(i)
	case *uint32:
		*numPtr.(*uint32) = uint32(i)
	case *uint64:
		*numPtr.(*uint64) = uint64(i)
	case *float32:
		*numPtr.(*float32) = float32(i)
	case *float64:
		*numPtr.(*float64) = float64(i)
	case *string:
		*numPtr.(*string) = strconv.FormatInt(i, 10)
	default:
		err := errors.New(fmt.Sprint("bad type:", reflect.TypeOf(numPtr))) // don't use zlog, will be import cycle
		fmt.Println("zint.SetAny err:", err)
		return err
	}
	return nil
}

func IsInt(i any) bool {
	switch i.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

func FromAnyInt(i any) (int64, error) {
	if i == nil {
		return 0, errors.New("is nil")
	}
	switch n := i.(type) {
	case int:
		return int64(n), nil
	case int8:
		return int64(n), nil
	case int16:
		return int64(n), nil
	case int32:
		return int64(n), nil
	case int64:
		return n, nil
	case uint:
		return int64(n), nil
	case uint8:
		return int64(n), nil
	case uint16:
		return int64(n), nil
	case uint32:
		return int64(n), nil
	case uint64:
		return int64(n), nil
	}
	val := reflect.ValueOf(i)
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(val.Int()), nil
	}
	return 0, fmt.Errorf("bad kind %v", reflect.ValueOf(i).Kind())
}

func GetAny(i any) (int64, error) {
	n, err := FromAnyInt(i)
	if err == nil {
		return n, nil
	}
	switch n := i.(type) {
	case bool:
		if n {
			return 1, nil
		}
		return 0, nil
	case float32:
		return int64(n), nil
	case float64:
		return int64(n), nil
	case string:
		sn, err := strconv.ParseInt(n, 10, 64)
		return sn, err
	}
	val := reflect.ValueOf(i)
	switch val.Kind() {
	case reflect.Bool:
		if val.Bool() {
			return 1, nil
		}
		return 0, nil
	case reflect.Float32, reflect.Float64:
		return int64(val.Float()), nil
	case reflect.String:
		n, err := strconv.ParseInt(val.String(), 10, 64)
		return n, err
	}
	return 0, fmt.Errorf("bad kind %v", reflect.ValueOf(i).Kind())
}

// GetItem makes Slice worth with MenuView MenuItems interface
func (s Slice) GetItem(i int) (id, name string, value any) {
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

func MakeHumanFriendly[N int | int64](n N) string {
	in := strconv.FormatInt(int64(n), 10)
	numOfDigits := len(in)
	if n < 0 {
		numOfDigits-- // First character is the - sign (not a digit)
	}
	numOfCommas := (numOfDigits - 1) / 3

	out := make([]byte, len(in)+numOfCommas)
	if n < 0 {
		in, out[0] = in[1:], '-'
	}

	for i, j, k := len(in)-1, len(out)-1, 0; ; i, j = i-1, j-1 {
		out[j] = in[i]
		if i == 0 {
			return string(out)
		}
		if k++; k == 3 {
			j, k = j-1, 0
			out[j] = ','
		}
	}
	return string(out)
}

func NumberToCircledNumbersString(n int) string {
	switch n {
	case 0:
		return "⓪"
	case 1:
		return "①"
	case 2:
		return "②"
	case 3:
		return "③"
	case 4:
		return "④"
	case 5:
		return "⑤"
	case 6:
		return "⑥"
	case 7:
		return "⑦"
	case 8:
		return "⑧"
	case 9:
		return "⑨"
	case 10:
		return "⑩"
	case 11:
		return "⑪"
	case 12:
		return "⑫"
	case 13:
		return "⑬"
	case 14:
		return "⑭"
	case 15:
		return "⑮"
	case 16:
		return "⑯"
	case 17:
		return "⑰"
	case 18:
		return "⑱"
	case 19:
		return "⑲"
	case 20:
		return "⑳"
	}
	var out string
	str := strconv.Itoa(n)
	for _, c := range str {
		cn := int(c - '0')
		out += NumberToCircledNumbersString(cn)
	}
	return out
}
