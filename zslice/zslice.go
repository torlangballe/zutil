package zslice

import (
	"errors"
	"reflect"

	"github.com/torlangballe/zutil/zlog"
)

func checkIsSlicePtr(s interface{}) error {
	if s == nil {
		return errors.New("slice is nil")
	}

	slicePtrValue := reflect.ValueOf(s)
	// should be pointer
	if slicePtrValue.Type().Kind() != reflect.Ptr {
		return errors.New("should be slice pointer")
	}

	sliceValue := slicePtrValue.Elem()
	// should be slice
	if sliceValue.Type().Kind() != reflect.Slice {
		return errors.New("should be slice pointer")
	}

	return nil
}

func RemoveAt(slice interface{}, index int) error {
	err := checkIsSlicePtr(slice)
	if err != nil {
		return zlog.Fatal(err, zlog.StackAdjust(1), "not slice pointer")
	}

	slicePtrValue := reflect.ValueOf(slice)
	sliceValue := slicePtrValue.Elem()
	if index < 0 || index >= sliceValue.Len() {
		return errors.New("index out of range")
	}
	sliceValue.Set(reflect.AppendSlice(sliceValue.Slice(0, index), sliceValue.Slice(index+1, sliceValue.Len())))
	return nil
}

func MixedValueAtIndexF64(array []float64, index float64) float64 {
	if index < 0.0 {
		return array[0]
	}
	if index >= float64(len(array))-1 {
		return array[len(array)-1]
	}
	n := index
	f := (index - n)
	var v = array[int(n)] * (1 - f)
	if int(n) < len(array) {
		v += array[int(n+1)] * f
		return v
	}
	if len(array) > 0 {
		return array[len(array)-1]
	}
	return 0
}

func MixedValueAtTForF64(array []float64, t float64) float64 {
	return MixedValueAtIndexF64(array, float64(len(array)-1)*t)
}

func Behead(slice interface{}) {
	RemoveAt(slice, 0)
}

