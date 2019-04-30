package ufloat

import (
	"errors"
	"fmt"
	"reflect"
)

func GetFloat64FromInterface(i interface{}) (float64, error) {
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
