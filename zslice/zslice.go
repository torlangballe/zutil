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
		return zlog.Fatal(err, zlog.StackAdjust(1), "not slice pointer", slice)
	}

	slicePtrValue := reflect.ValueOf(slice)
	sliceValue := slicePtrValue.Elem()
	if index < 0 || index >= sliceValue.Len() {
		return zlog.Error(nil, "index out of range:", index)
	}
	sliceValue.Set(reflect.AppendSlice(sliceValue.Slice(0, index), sliceValue.Slice(index+1, sliceValue.Len())))
	return nil
}

func Behead(slice interface{}) {
	RemoveAt(slice, 0)
}

func RemoveIf(slice interface{}, remove func(i int) bool) {
	for {
		val := reflect.ValueOf(slice).Elem()
		len := val.Len()
		found := false
		for i := 0; i < len; i++ {
			if remove(i) {
				RemoveAt(slice, i)
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
}

func CopyTo(slice, to interface{}) {
	toVal := reflect.ValueOf(to)
	sliceVal := reflect.ValueOf(slice)
	toVal.SetCap(sliceVal.Len())
	reflect.Copy(toVal, sliceVal)
}
