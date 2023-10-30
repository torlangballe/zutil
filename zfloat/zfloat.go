package zfloat

import (
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
)

type Range struct {
	Min float64
	Max float64
}

type Slice []float64

func RangeF(min, max float64) Range {
	return Range{Min: min, Max: max}
}

func GetAny(i interface{}) (float64, error) {
	switch i.(type) {
	case bool:
		if i.(bool) {
			return 1, nil
		}
		return 0, nil
	case int:
		return float64(i.(int)), nil
	case int8:
		return float64(i.(int8)), nil
	case int16:
		return float64(i.(int16)), nil
	case int32:
		return float64(i.(int32)), nil
	case int64:
		return float64(i.(int64)), nil
	case uint:
		return float64(i.(uint)), nil
	case uint8:
		return float64(i.(uint8)), nil
	case uint16:
		return float64(i.(uint16)), nil
	case uint32:
		return float64(i.(uint32)), nil
	case uint64:
		return float64(i.(uint64)), nil
	case float32:
		return float64(i.(float32)), nil
	case float64:
		return float64(i.(float64)), nil
	case string:
		f, err := strconv.ParseFloat(i.(string), 64)
		return f, err
	}
	val := reflect.ValueOf(i)
	switch val.Kind() {
	case reflect.Bool:
		if i.(bool) {
			return 1, nil
		}
		return 0, nil
	case reflect.Int:
		return float64(val.Int()), nil
	case reflect.Int8:
		return float64(val.Int()), nil
	case reflect.Int16:
		return float64(val.Int()), nil
	case reflect.Int32:
		return float64(val.Int()), nil
	case reflect.Int64:
		return float64(val.Int()), nil
	case reflect.Uint:
		return float64(val.Int()), nil
	case reflect.Uint8:
		return float64(val.Int()), nil
	case reflect.Uint16:
		return float64(val.Int()), nil
	case reflect.Uint32:
		return float64(val.Int()), nil
	case reflect.Uint64:
		return float64(val.Int()), nil
	case reflect.Float32:
		return float64(val.Float()), nil
	case reflect.Float64:
		return float64(val.Float()), nil
	case reflect.String:
		n, err := strconv.ParseFloat(val.String(), 64)
		return n, err
	default:
		return 0, errors.New(fmt.Sprint("bad type:", reflect.TypeOf(i)))
	}
}

func SetAny(num interface{}, f float64) error {
	// zlog.Info("SetAnyF:", reflect.ValueOf(y).Type())
	switch atype := num.(type) {
	case *float32:
		*atype = float32(f)
	case *float64:
		*num.(*float64) = f
	case bool:
		*num.(*bool) = (f != 0)
	case int:
		*num.(*int) = int(f)
	case int8:
		*num.(*int8) = int8(f)
	case int16:
		*num.(*int16) = int16(f)
	case int32:
		*num.(*int32) = int32(f)
	case int64:
		*num.(*int64) = int64(f)
	case uint:
		*num.(*uint) = uint(f)
	case uint8:
		*num.(*uint8) = uint8(f)
	case uint16:
		*num.(*uint16) = uint16(f)
	case uint32:
		*num.(*uint32) = uint32(f)
	case uint64:
		*num.(*uint64) = uint64(f)

	default:
		return errors.New(fmt.Sprint("bad type:", reflect.TypeOf(num)))
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

func Float64Max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func Float64Min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// GetItem makes Slice worth with MenuView MenuItems interface
func (s Slice) GetItem(i int) (id, name string, value interface{}) {
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
