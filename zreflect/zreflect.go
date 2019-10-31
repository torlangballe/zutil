package zreflect

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
)

var timeType = reflect.TypeOf(time.Time{})

type TypeKind string

const (
	KindUndef  TypeKind = "undef"
	KindString          = "string"
	KindInt             = "int"
	KindFloat           = "float"
	KindBool            = "bool"
	KindStruct          = "struct"
	KindTime            = "time"
	KindByte            = "byte"
	KindMap             = "map"
	KindFunc            = "function"
)

type FieldTag struct {
	Label string
	Vars  []string
}

func GetTagAsFields(stag string) []FieldTag {
	if stag != "" {
		re, _ := regexp.Compile(`(\w+)\s*:"([^"]*)"\s*`) // http://regoio.herokuapp.com
		matches := re.FindAllStringSubmatch(stag, -1)
		if len(matches) > 0 {
			fields := make([]FieldTag, len(matches))
			for i, groups := range matches {
				var ft FieldTag
				ft.Label = groups[1]
				ft.Vars = strings.Split(groups[2], ",")
				//						fmt.Println("stagmatch:", ft.Label, ft.Vars)
				fields[i] = ft
			}
			return fields
		}
	}
	return nil
}

type Item struct {
	Kind        TypeKind
	TypeName    string
	FieldName   string
	Tag         string
	IsAnonymous bool
	IsArray     bool
	IsPointer   bool
	Address     interface{}
	Package     string
	Value       reflect.Value
	Interface   interface{}
	Children    []Item
}

func itterate(fieldName, typeName, tagName string, isAnonymous, unnestAnonymous bool, val reflect.Value) (item Item, err error) {
	item.FieldName = fieldName
	vtype := val.Type()
	if typeName == "" {
		typeName = vtype.Name()
	}
	item.TypeName = typeName
	item.Package = vtype.PkgPath()
	if !val.IsValid() {
		err = errors.Errorf("marshalValue: val.IsValid() failed")
		return
	}
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		t := vtype
		if val.IsNil() {
			t = vtype.Elem()
			val = reflect.Zero(t)
		} else {
			val = reflect.Indirect(val)
		}
		//		fmt.Println("ptr:", t.Name(), fieldName, t.PkgPath())
		item, err = itterate(fieldName, t.Name(), tagName, isAnonymous, unnestAnonymous, val)
		item.IsPointer = true
		return

	case reflect.Slice:
		t := reflect.TypeOf(val.Interface()).Elem()
		v := reflect.Zero(t) // something wrong here...
		if t.Kind() == reflect.Ptr {
		}
		//		fmt.Println("slice:", t.Name(), fieldName, t.PkgPath())
		item, err = itterate(fieldName, t.Name(), tagName, isAnonymous, unnestAnonymous, v)
		item.IsArray = true
		item.Kind = KindString
		item.Interface = val.Interface()
		//		fmt.Println("item slice:", item.IsArray)
		return

	case reflect.Array:
		return

	case reflect.Struct:
		switch vtype {
		case timeType:
			item.Kind = KindTime
			item.Interface = val.Interface()
		default:
			item.Interface = val.Interface()
			item.Kind = KindStruct
			n := vtype.NumField()
			for i := 0; i < n; i++ {
				f := vtype.Field(i)
				fval := val.Field(i)
				tag := string(f.Tag) // http://golang.org/pkg/reflect/#StructTag
				tname := fval.Type().Name()
				c, e := itterate(f.Name, tname, tag, f.Anonymous, unnestAnonymous, fval)
				//				fmt.Println("struct:", f.Type, f.Name, f.Type.PkgPath(), tag, c.Tag)
				if e != nil {
					err = e
				} else {
					if fval.CanAddr() {
						c.Address = fval.Addr().Interface()
					}
					if unnestAnonymous && f.Anonymous {
						item.Children = append(item.Children, c.Children...)
					} else {
						item.Children = append(item.Children, c)
					}
				}
			}
		}

	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8:
		item.Kind = KindInt
		item.Interface = val.Int()

	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		item.Kind = KindInt
		item.Interface = val.Uint()

	case reflect.String:
		item.Kind = KindString
		item.Interface = val.String()

	case reflect.Bool:
		item.Kind = KindBool
		item.Interface = val.Bool()

	case reflect.Float32, reflect.Float64:
		item.Kind = KindFloat
		item.Interface = val.Float()

	case reflect.Map:
		item.Kind = KindTime
		item.Interface = val.Interface()

	case reflect.Func:
		item.Kind = KindFunc
		item.Interface = val.Interface()

	default:
		fmt.Println("marshal.marshalValue: Carefull, unknown type!", val.Kind())
		item.Kind = KindUndef
	}
	item.FieldName = fieldName
	item.IsAnonymous = isAnonymous
	item.Tag = tagName
	item.Value = val
	return
}

func ItterateStruct(istruct interface{}, unnestAnonymous bool) (item Item, err error) {
	return itterate("", "", "", false, unnestAnonymous, reflect.ValueOf(istruct).Elem())
}
