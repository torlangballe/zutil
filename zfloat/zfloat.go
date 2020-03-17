package zfloat

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
)

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
	default:
		return 0, errors.New(fmt.Sprint("bad type:", reflect.TypeOf(i)))
	}
}

func SetAny(any interface{}, f float64) error {
	switch any.(type) {
	case float32:
		*any.(*float32) = float32(f)
	case float64:
		*any.(*float64) = f
	case bool:
		*any.(*bool) = (f != 0)
	case int:
		*any.(*int) = int(f)
	case int8:
		*any.(*int8) = int8(f)
	case int16:
		*any.(*int16) = int16(f)
	case int32:
		*any.(*int32) = int32(f)
	case int64:
		*any.(*int64) = int64(f)
	case uint:
		*any.(*uint) = uint(f)
	case uint8:
		*any.(*uint8) = uint8(f)
	case uint16:
		*any.(*uint16) = uint16(f)
	case uint32:
		*any.(*uint32) = uint32(f)
	case uint64:
		*any.(*uint64) = uint64(f)

	default:
		return errors.New(fmt.Sprint("bad type:", reflect.TypeOf(any)))
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

func AddToSet(n float64, slice *[]float64) bool {
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
