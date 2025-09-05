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

func ValueFromTypeFormatSuffixedName(fullName string, in any) (a any, fname, ftype, tags string, err error) {
	fullName = zstr.HeadUntilWithRest(fullName, "|", &tags)
	if !zstr.SplitN(fullName, ":", &fname, &ftype) {
		return nil, "", "", "", errors.New("Not found")
	}
	n, _, got := DefaultTypeRegistrar.NewForType(ftype)
	if !got {
		return nil, "", "", "", nil
	}
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			fmt.Println(err, ftype)
			return nil, "", "", "", err
		}
		err = json.Unmarshal(data, n)
		if err != nil {
			fmt.Println(err, ftype)
			return nil, "", "", "", err
		}
	}
	return n, fname, ftype, tags, nil
}
