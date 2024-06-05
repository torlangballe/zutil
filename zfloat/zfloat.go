package zfloat

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
)

type Range struct {
	Min float64
	Max float64
}

type Slice []float64

func RangeF(min, max float64) Range {
	return Range{Min: min, Max: max}
}

func GetAny(i any) (float64, error) {
	switch n := i.(type) {
	case bool:
		if n {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	case float32:
		return float64(n), nil
	case float64:
		return n, nil
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err
	}
	val := reflect.ValueOf(i)
	switch val.Kind() { // not sure this does anything type switch above does...
	case reflect.Bool:
		if val.Bool() {
			return 1, nil
		}
		return 0, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(val.Int()), nil
	case reflect.Float32, reflect.Float64:
		return float64(val.Float()), nil
	case reflect.String:
		n, err := strconv.ParseFloat(val.String(), 64)
		return n, err
	}
	return 0, errors.New(fmt.Sprint("zfloat.GetAny bad type:", reflect.TypeOf(i)))
}

func SetAny(num any, f float64) error {
	switch n := num.(type) {
	case *float32:
		*n = float32(f)
	case *float64:
		*n = f
	case *string:
		*n = strconv.FormatFloat(f, 'f', -1, 64)
	case *bool:
		*n = (f != 0)
	case *int:
		*n = int(f)
	case *int8:
		*n = int8(f)
	case *int16:
		*n = int16(f)
	case *int32:
		*n = int32(f)
	case *int64:
		*n = int64(f)
	case *uint:
		*n = uint(f)
	case *uint8:
		*n = uint8(f)
	case *uint16:
		*n = uint16(f)
	case *uint32:
		*n = uint32(f)
	case *uint64:
		*n = uint64(f)
	default:
		return errors.New(fmt.Sprint("zfloat.SetAny bad type:", reflect.TypeOf(num)))
	}
	return nil
}

func IsInSlice(n float64, slice []float64) bool {
	for _, s := range slice {
		if s == n {
			return true
		}
	}
	return false
}

func AddToSet(slice *[]float64, n float64) bool {
	if !IsInSlice(n, *slice) {
		*slice = append(*slice, n)
		return true
	}
	return false
}

func Max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func Min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func Minimize32(a *float32, b float32) bool {
	if *a < b {
		return false
	}
	*a = b
	return true
}

func Maximize32(a *float32, b float32) bool {
	if *a > b {
		return false
	}
	*a = b
	return true
}

func Minimize(a *float64, b float64) bool {
	if *a < b {
		return false
	}
	*a = b
	return true
}

func Maximize(a *float64, b float64) bool {
	if *a > b {
		return false
	}
	*a = b
	return true
}

// GetItem makes Slice worth with MenuView MenuItems interface
func (s Slice) GetItem(i int) (id, name string, value any) {
	if i >= len(s) {
		return "", "", nil
	}
	n := s[i]
	str := strconv.FormatFloat(n, 'f', -1, 64)
	return str, str, n
}

func (s Slice) Count() int {
	return len(s)
}

func (s Slice) Minimum() float64 {
	var min float64
	for i, n := range s {
		if i == 0 || min > n {
			min = n
		}
	}
	return min
}

func Clamped(v, min, max float64) float64 {
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

func Clamped32(v, min, max float32) float32 {
	if v < min {
		v = min
	}
	if v > max {
		v = max
	}
	return v
}

// MixedValueAtT returns the mix beween the values t lies within using MixedValueAtIndex
// t is 0-1, corresponding to 0 as 100% of [0] and 1 as 100% the last slice element.
func MixedValueAtT(slice []float64, t float64) float64 {
	return MixedValueAtIndex(slice, float64(len(slice)-1)*t)
}

// MixedValueAtIndex returns a mix of the value before and after index;
// So if index is 4.25, it will return a mix of 75% [4] and 25% [5]
func MixedValueAtIndex(slice []float64, index float64) float64 {
	if index < 0.0 {
		return slice[0]
	}
	if index >= float64(len(slice))-1 {
		return slice[len(slice)-1]
	}
	n := index
	f := (index - n)
	var v = slice[int(n)] * (1 - f)
	if int(n) < len(slice) {
		v += slice[int(n+1)] * f
		return v
	}
	if len(slice) > 0 {
		return slice[len(slice)-1]
	}
	return 0
}

func MaxKeyOfMap(m map[string]float64) (key string, value float64) {
	value = -math.MaxFloat64
	for k, v := range m {
		if v > value {
			value = v
			key = k
		}
	}
	return
}

func Join64(ids []int64, sep string) string {
	var str string
	for i, id := range ids {
		if i != 0 {
			str += sep
		}
		str += strconv.FormatInt(id, 10)
	}
	return str
}

func SplitStringTo64(str string, sep string) []float64 {
	if len(str) == 0 {
		return nil
	}
	return StringsTo64(strings.Split(str, sep))
}

func StringsTo64(snums []string) []float64 {
	var floats []float64

	for _, s := range snums {
		s = strings.TrimSpace(s)
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			floats = append(floats, f)
		}
	}
	return floats
}

func ToStrings64(floats []float64) (s []string) {
	for _, n := range floats {
		s = append(s, strconv.FormatFloat(n, 'f', -1, 64))
	}
	return
}
