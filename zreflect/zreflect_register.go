package zreflect

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
)

var DefaultTypeRegistrar TypeRegistrar[*struct{}]

func MakeTypeNameWithType(s any) string {
	rtype := reflect.TypeOf(s)
	return MakeTypeNameWithPackage(rtype)
}

func MakeTypeNameWithPackage(rtype reflect.Type) string {
	_, pkg := path.Split(rtype.PkgPath())
	if pkg != "" {
		return pkg + "." + rtype.Name()
	}
	return strings.TrimLeft(rtype.String(), "[]*")
}

type regRow[S any] struct {
	rtype reflect.Type
	info  S
}
type TypeRegistrar[I any] struct {
	m zmap.LockMap[string, regRow[I]]
}

func (r *TypeRegistrar[I]) Register(structure any, info I) string {
	rtype := reflect.TypeOf(structure)
	typeName := MakeTypeNameWithPackage(rtype)
	// fmt.Println("RegisterCreatorForType:", typeName, rtype)
	row := regRow[I]{rtype: rtype, info: info}
	r.m.Set(typeName, row)
	return typeName
}

func (r *TypeRegistrar[I]) Lookup(typeName string) (rtype reflect.Type, info I, got bool) {
	row, got := r.m.Get(typeName)
	if !got {
		return rtype, info, false
	}
	return row.rtype, row.info, true
}

func (r *TypeRegistrar[I]) NewForType(typeName string) (aPtr any, info I, got bool) {
	rtype, info, got := r.Lookup(typeName)
	if !got {
		return nil, info, false
	}
	n := reflect.New(rtype).Interface()
	// fmt.Println("NewForRegisteredType:", typeName, rtype, reflect.TypeOf(n))
	return n, info, true
}

func splitN(str, sep string, a, b *string) bool { // we can't use zstr.SplitN as it's cyclical
	parts := strings.Split(str, sep)
	if len(parts) != 2 {
		return false
	}
	*a = parts[0]
	*b = parts[1]
	return true
}

func NewTypeFromRegisteredTypeName(typeName string, initWithVal any) (a any, tname, tag string, err error) {
	splitN(typeName, "|", &tname, &tag)
	n, _, got := DefaultTypeRegistrar.NewForType(tname)
	if got {
		if initWithVal != nil {
			data, err := json.Marshal(initWithVal)
			if err != nil {
				fmt.Println(err, tname)
				return nil, "", "", err
			}
			err = json.Unmarshal(data, n)
			if err != nil {
				fmt.Println(err, tname)
				return nil, "", "", err
			}
		}
		return n, tag, tname, nil
	}
	return nil, "", "", errors.New("Not found: " + typeName)
}

func ValueFromTypeFormatPrefixedName(fullName string, in any) (a any, fname, ftype, tags string, err error) {
	var rest string
	fname = zstr.HeadUntilWithRest(fullName, ":", &rest)
	if rest == "" {
		return nil, "", "", "", nil
	}
	a, ftype, tags, err = NewTypeFromRegisteredTypeName(rest, in)
	return a, fname, ftype, tags, err
}
