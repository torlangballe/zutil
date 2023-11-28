package zslice

import (
	"errors"
	"reflect"

	"github.com/torlangballe/zutil/zlog"
)

// https://github.com/golang/go/wiki/SliceTricks#delete

func CheckIsSlicePtr(s interface{}) error {
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
	err := CheckIsSlicePtr(slice)
	if err != nil {
		return zlog.Fatal(err, zlog.StackAdjust(1), "not slice pointer", slice, reflect.TypeOf(slice).Kind(), reflect.TypeOf(slice))
	}

	slicePtrValue := reflect.ValueOf(slice)
	sliceValue := slicePtrValue.Elem()
	if index < 0 || index >= sliceValue.Len() {
		return zlog.Error(nil, "index out of range:", index)
	}
	sliceValue.Set(reflect.AppendSlice(sliceValue.Slice(0, index), sliceValue.Slice(index+1, sliceValue.Len())))
	return nil
}

func Empty(slicePtr interface{}) {
	rval := reflect.ValueOf(slicePtr).Elem()
	for {
		length := rval.Len()
		if length == 0 {
			break
		}
		RemoveAt(slicePtr, length-1)
	}
}

func AddEmptyElementAtEnd(slicePtr interface{}) int {
	e := MakeAnElementOfSliceType(slicePtr)
	return AddAtEnd(slicePtr, e)
}

func MakeAnElementOfSliceRValType(rval reflect.Value) reflect.Value {
	if rval.Kind() == reflect.Pointer {
		rval = rval.Elem()
	}
	// zlog.Info("MakeAnElementOfSliceType:", rval.Type(), rval.Kind())
	// return reflect.New(rval.Type()).Elem().Interface()
	return reflect.New(rval.Type().Elem()).Elem()
}

func MakeAnElementOfSliceType(slice any) any {
	return MakeAnElementOfSliceRValType(reflect.ValueOf(slice)).Interface()
}

func AddAtEnd(slicePtr interface{}, add interface{}) int {
	rptr := reflect.ValueOf(slicePtr)
	rval := rptr.Elem()
	rval = reflect.Append(rval, reflect.ValueOf(add))
	rptr.Elem().Set(rval)
	return rval.Len() - 1
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

func CopyTo(to, slice interface{}) {
	sliceVal := reflect.ValueOf(slice)
	destVal := reflect.MakeSlice(sliceVal.Type(), sliceVal.Len(), sliceVal.Len())
	reflect.Copy(destVal, sliceVal)
	toVal := reflect.ValueOf(to)
	toVal.Elem().Set(destVal)
}

func IndexOf(length int, is func(i int) bool) int {
	for i := 0; i < length; i++ {
		if is(i) {
			return i
		}
	}
	return -1
}

// func Reverse[T interface{}](s []T) {
// 	first := 0
// 	last := len(s) - 1
// 	for first < last {
// 		s[first], s[last] = s[last], s[first]
// 		first++
// 		last--
// 	}
// }

func Reverse(s interface{}) {
	n := reflect.ValueOf(s).Len()
	swap := reflect.Swapper(s)
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		swap(i, j)
	}
}

func Swap[A any](slice []A, i, j int) {
	t := slice[i]
	slice[i] = slice[j]
	slice[j] = t
}

func Add[T any](s *[]T, a T) {
	*s = append(*s, a)
}

func Reduced[A any](slice []A, keep func(a A) bool) []A {
	var snew []A
	for i, s := range slice {
		if keep(slice[i]) {
			snew = append(snew, s)
		}
	}
	return snew
}
