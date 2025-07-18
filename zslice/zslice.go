package zslice

import (
	"errors"
	"math/rand"
	"reflect"
	"slices"

	"github.com/torlangballe/zutil/zdebug"
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
		return zlog.Error("index out of range:", index, sliceValue.Len(), zdebug.CallingStackString())
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
	return RValAddAtEnd(reflect.ValueOf(slicePtr), reflect.ValueOf(add))
}

func RValAddAtEnd(slicePtr, add reflect.Value) int {
	rval := slicePtr.Elem()
	rval = reflect.Append(rval, add)
	slicePtr.Elem().Set(rval)
	return rval.Len() - 1
}

func Behead(slice any) {
	RemoveAt(slice, 0)
}

func Delete[S comparable](s *[]S, dels ...S) int {
	count := 0
	for i := 0; i < len(*s); {
		if slices.Contains(dels, (*s)[i]) {
			count++
			RemoveAt(s, i)
			break
		} else {
			i++
		}
	}
	return count
}

func Deleted[S comparable](slice []S, dels ...S) []S {
	var out []S
	for _, s := range slice {
		if !slices.Contains(dels, s) {
			out = append(out, s)
		}
	}
	return out
}

func DeleteFromFunc[S any](s *[]S, del func(s S) bool) {
	for i := 0; i < len(*s); {
		if del((*s)[i]) {
			RemoveAt(s, i)
			continue
		}
		i++
	}
}

func CopyTo(toPtr, slice any) {
	sliceVal := reflect.ValueOf(slice)
	destVal := reflect.MakeSlice(sliceVal.Type(), sliceVal.Len(), sliceVal.Len())
	reflect.Copy(destVal, sliceVal)
	toVal := reflect.ValueOf(toPtr)
	toVal.Elem().Set(destVal)
}

func NewCopy(slice any) any {
	sliceVal := reflect.ValueOf(slice)
	if sliceVal.Type().Elem().Kind() == reflect.Interface {
		zlog.Info("NewCopy:", sliceVal.Type().Elem().Kind(), sliceVal.Type().Elem().Elem().Kind())
		// sliceVal = sliceVal.Type().Elem()
	}
	destVal := reflect.MakeSlice(sliceVal.Type(), sliceVal.Len(), sliceVal.Len())
	destNew := reflect.New(destVal.Type())
	destNew.Elem().Set(destVal)
	reflect.Copy(destNew.Elem(), sliceVal)
	return destNew.Interface()
}

func IndexOf(length int, is func(i int) bool) int {
	for i := 0; i < length; i++ {
		if is(i) {
			return i
		}
	}
	return -1
}

func Find[S any](slice []S, is func(s S) bool) (*S, int) {
	i := slices.IndexFunc(slice, is)
	if i == -1 {
		return nil, -1
	}
	return &slice[i], i
}

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

func AddToSet[T comparable](s *[]T, adds ...T) {
	for _, a := range adds {
		if !slices.Contains(*s, a) {
			Add(s, a)
		}
	}
}

func Union[T comparable](a, b []T) []T {
	n := slices.Clone(a)
	for _, ib := range b {
		AddToSet(&n, ib)
	}
	return n
}

// Exclusion returns elements that are not BOTH in a and b
func Exclusion[T comparable](a, b []T) []T { // todo: Done in slices already???
	var x []T
	n := slices.Clone(a)
	for _, ib := range b {
		i := slices.Index(n, ib)
		if i == -1 {
			x = append(x, ib)
		} else {
			RemoveAt(&n, i)
			i--
		}
	}
	return append(x, n...)
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

func Random[S any](slice []S) S {
	i := RandomIndex(slice)
	return slice[i]
}

func RandomIndex[S any](slice []S) int {
	zlog.Assert(len(slice) != 0)
	i := rand.Int31n(int32(len(slice)))
	return int(i)
}

func SplitWithFunc[S any](slice []S, match func(s S) bool) (is []S, not []S) {
	for _, s := range slice {
		if match(s) {
			is = append(is, s)
		} else {
			not = append(not, s)
		}
	}
	return is, not
}

func Map[S any](slice []S, mapFunc func(s S) S) []S {
	out := make([]S, len(slice))
	for i, s := range slice {
		out[i] = mapFunc(s)
	}
	return out
}

func AddOrReplace[S any](slice *[]S, add S, equal func(a, b S) bool) {
	for i, s := range *slice {
		if equal(add, s) {
			(*slice)[i] = add
			return
		}
	}
	*slice = append(*slice, add)
}

func InsertSorted[S any](slice []S, insert S, less func(a, b S) bool) []S {
	for i, s := range slice {
		if less(s, insert) {
			return slices.Insert(slice, i, insert)
		}
	}
	return append(slice, insert)
}

func Pop[S any](s *[]S) S {
	slen := len(*s)
	zlog.Assert(slen > 0)
	top := (*s)[slen-1]
	*s = (*s)[:slen-1]
	return top
}

func Top[S any](s []S) S {
	slen := len(s)
	zlog.Assert(slen > 0)
	return (s)[slen-1]
}
