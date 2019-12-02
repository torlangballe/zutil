package zfloat

import (
	"errors"
	"fmt"
	"reflect"
)

func GetAnyFloat(i interface{}) (float64, error) {
	switch i.(type) {
	case int:
		return float64(i.(int)), nil
	case int32:
		return float64(i.(int32)), nil
	case int64:
		return float64(i.(int64)), nil
	case float32:
		return float64(i.(float32)), nil
	case float64:
		return float64(i.(float64)), nil
	default:
		return 0, errors.New(fmt.Sprint("bad type:", reflect.TypeOf(i)))
	}
}

func SetAnyFloat(any interface{}, f float64) error {
	switch any.(type) {
	case float32:
		*any.(*float32) = float32(f)
	case float64:
		*any.(*float64) = f
	default:
		return errors.New(fmt.Sprint("bad type:", reflect.TypeOf(any)))
	}
	return nil
}

func IsFloat64InSlice(n float64, slice []float64) bool {
	for _, s := range slice {
		if s == n {
			return true
		}
	}
	return false
}

func AddIntFloat64ToSet(n float64, slice *[]float64) bool {
	if !IsFloat64InSlice(n, *slice) {
		*slice = append(*slice, n)
		return true
	}
	return false
}

func Float32Max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func Float32Min(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func Float32Minimize(a *float32, b float32) bool {
	if *a < b {
		return false
	}
	*a = b
	return true
}

func Float32Maximize(a *float32, b float32) bool {
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
