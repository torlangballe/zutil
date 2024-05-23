package zslice

import (
	"errors"
	"math/rand"
	"reflect"
	"slices"

	"github.com/torlangballe/zutil/zlog"
)

// https://github.com/golang/go/wiki/SliceTricks#delete

func CheckIsSlicePtr(s any) error {
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

func RemoveAt(slice any, index int) error {
	err := CheckIsSlicePtr(slice)
	if err != nil {
		return zlog.Fatal(err, zlog.StackAdjust(1), "not slice pointer", slice, reflect.TypeOf(slice).Kind(), reflect.TypeOf(slice))
	}

	slicePtrValue := reflect.ValueOf(slice)
	sliceValue := slicePtrValue.Elem()
	if index < 0 || index >= sliceValue.Len() {
		return zlog.Error("index out of range:", index)
	}
	sliceValue.Set(reflect.AppendSlice(sliceValue.Slice(0, index), sliceValue.Slice(index+1, sliceValue.Len())))
	return nil
}

func Empty(slicePtr any) {
	rval := reflect.ValueOf(slicePtr).Elem()
	for {
		length := rval.Len()
		if length == 0 {
			break
		}
		RemoveAt(slicePtr, length-1)
	}
}

func AddEmptyElementAtEnd(slicePtr any) int {
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

func AddAtEnd(slicePtr any, add any) int {
	rptr := reflect.ValueOf(slicePtr)
	rval := rptr.Elem()
	rval = reflect.Append(rval, reflect.ValueOf(add))
	rptr.Elem().Set(rval)
	return rval.Len() - 1
}

func Behead(slice any) {
	RemoveAt(slice, 0)
}

func DeleteFromFunc[S any](s *[]S, del func(s S) bool) {
	for i := 0; i < len(*s); {
		if del((*s)[i]) {
			RemoveAt(s, i)
		} else {
			i++
		}
	}
}

func CopyTo(to, slice any) {
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

// func Reverse[T any](s []T) {
// 	first := 0
// 	last := len(s) - 1
// 	for first < last {
// 		s[first], s[last] = s[last], s[first]
// 		first++
// 		last--
// 	}
// }

func Reverse(s any) {
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

func AddToSet[T comparable](s *[]T, a T) {
	if !slices.Contains(*s, a) {
		Add(s, a)
	}
}

func Union[T comparable](a, b []T) []T {
	n := slices.Clone(a)
	for _, ib := range b {
		AddToSet(&n, ib)
	}
	return n
}

func SetsOverlap[S comparable](a, b []S) bool {
	for _, ib := range b {
		if slices.Contains(a, ib) {
			return true
		}
	}
	return false
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

func Random[S any](slice []S) (S, int) {
	i := rand.Int31n(int32(len(slice)))
	return slice[i], int(i)
}

// func AppendAnyToAny(toPtr any, from any) error {
// 	fval := reflect.ValueOf(from)
// 	tval := reflect.ValueOf(toPtr).Elem()
// 	if fval.Kind() != reflect.Slice {
// 		return errors.New("from not slice")
// 	}
// 	if tval.Kind() != reflect.Slice {
// 		return errors.New("toPtr not to slice")
// 	}
// 	tv := tval
// 	for i := 0; i < fval.Len(); i++ {
// 		e := MakeAnElementOfSliceRValType(tval)
// 		zreflect.SetAnyToAny(e.Addr(), fval.Index(i))
// 		tv = reflect.Append(tv, e)
// 	}
// 	tval.Set(tv)
// }
