package zreflect

import (
	"encoding/json"
	"fmt"
	"path"
	"reflect"

	"github.com/torlangballe/zutil/zmap"
)

var DefaultStructRegistrar StructRegistrar[*struct{}]

func MakeTypeNameWithStruct(s any) string {
	rtype := reflect.TypeOf(s)
	return MakeTypeNameWithPackage(rtype)
}

func MakeTypeNameWithPackage(rtype reflect.Type) string {
	_, pkg := path.Split(rtype.PkgPath())
	return pkg + "." + rtype.Name()
}

type regRow[S any] struct {
	rtype reflect.Type
	info  S
}
type StructRegistrar[I any] struct {
	m zmap.LockMap[string, regRow[I]]
}

func (r *StructRegistrar[I]) Register(structure any, info I) string {
	rtype := reflect.TypeOf(structure)
	typeName := MakeTypeNameWithPackage(rtype)
	// fmt.Println("RegisterCreatorForType:", typeName, rtype)
	row := regRow[I]{rtype: rtype, info: info}
	r.m.Set(typeName, row)
	return typeName
}

func (r *StructRegistrar[I]) Lookup(typeName string) (rtype reflect.Type, info I, got bool) {
	row, got := r.m.Get(typeName)
	if !got {
		return rtype, info, false
	}
	return row.rtype, row.info, true
}

func (r *StructRegistrar[I]) NewForType(typeName string) (aPtr any, info I, got bool) {
	rtype, info, got := r.Lookup(typeName)
	if !got {
		return nil, info, false
	}
	n := reflect.New(rtype).Interface()
	fmt.Println("NewForRegisteredType:", typeName, rtype, reflect.TypeOf(n))
	return n, info, true
}

func (r *StructRegistrar[I]) SetRValFromRegisteredType(setInRVal *reflect.Value, typeName string) (info I, err error) {
	aPtr, info, got := r.NewForType(typeName)
	if !got {
		fmt.Println(err, typeName)
		return info, fmt.Errorf("Type not found: %v", typeName)
	}
	data, err := json.Marshal(setInRVal.Interface())
	if err != nil {
		fmt.Println(err, typeName)
		return info, err
	}
	err = json.Unmarshal(data, aPtr)
	if err != nil {
		fmt.Println(err, typeName)
		return info, err
	}
	*setInRVal = reflect.ValueOf(aPtr).Elem()
	return info, nil
}
