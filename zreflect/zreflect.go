package zreflect

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zlog"
)

var timeType = reflect.TypeOf(time.Time{})

type TypeKind string

const (
	KindUndef     TypeKind = "undef"
	KindString             = "string"
	KindInt                = "int"
	KindFloat              = "float"
	KindBool               = "bool"
	KindStruct             = "struct"
	KindTime               = "time"
	KindByte               = "byte"
	KindMap                = "map"
	KindFunc               = "function"
	KindSlice              = "slice"
	KindInterface          = "interface"
)

// GetTagAsFields returns a map of label:[vars] `json:"id, omitempty"` -> json : [id, omitempty]
func GetTagAsMap(stag string) map[string][]string {
	if stag != "" {
		m := map[string][]string{}
		re, _ := regexp.Compile(`(\w+)\s*:"([^"]*)"\s*`) // http://regoio.herokuapp.com
		matches := re.FindAllStringSubmatch(stag, -1)
		if len(matches) > 0 {
			for _, groups := range matches {
				label := groups[1]
				vars := strings.Split(groups[2], ",")
				m[label] = vars
			}
			return m
		}
	}
	return nil
}

type Item struct {
	Kind            TypeKind
	TypeName        string
	FieldName       string
	Tag             string
	IsAnonymous     bool
	IsArray         bool
	IsPointer       bool
	Address         interface{}
	Package         string
	Value           reflect.Value
	Interface       interface{}
	SimpleInterface interface{}
	Children        []Item
}

func itterate(level int, fieldName, typeName, tagName string, isAnonymous, unnestAnonymous, recursive bool, val reflect.Value) (item Item, err error) {
	//    zlog.Info("itterate:", level, fieldName, typeName, tagName)
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
	case reflect.Interface:
		item.Interface = val.Interface()
		item.Kind = KindInterface

	case reflect.Ptr:
		t := vtype
		if val.IsNil() {
			t = vtype.Elem()
			val = reflect.Zero(t)
		} else {
			val = reflect.Indirect(val)
		}
		//		zlog.Info("ptr:", t.Name(), fieldName, t.PkgPath())
		item, err = itterate(level, fieldName, t.Name(), tagName, isAnonymous, unnestAnonymous, recursive, val)
		item.IsPointer = true
		//		item.Address = val.Addr().Interface()
		return

	case reflect.Slice:
		t := reflect.TypeOf(val.Interface()).Elem()
		v := reflect.Zero(t)
		if !v.CanAddr() {
			v = reflect.New(t)
		}
		// zlog.Info("slice:", val.Len(), t.Name(), fieldName, t.PkgPath(), v.Kind())
		item, err = itterate(level, fieldName, t.Name(), tagName, isAnonymous, unnestAnonymous, recursive, v)
		item.Value = val
		item.IsArray = true
		item.Kind = KindSlice
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()
		return

	case reflect.Array:
		return

	case reflect.Struct:
		switch vtype {
		case timeType:
			item.Kind = KindTime
			item.Interface = val.Interface()
			item.SimpleInterface = val.Interface()
		default:
			item.Interface = val.Interface()
			item.SimpleInterface = val.Interface()
			item.Kind = KindStruct
			n := vtype.NumField()
			// zlog.Info("struct:", fieldName, n, recursive, unnestAnonymous , isAnonymous)
			if level > 0 && !recursive && (!unnestAnonymous || !isAnonymous) { // always go into first level
				break
			}
			for i := 0; i < n; i++ {
				f := vtype.Field(i)
				fval := val.Field(i)
				tag := string(f.Tag) // http://golang.org/pkg/reflect/#StructTag
				tname := fval.Type().Name()
				fname := f.Name
				// if unnestAnonymous && fieldName == "" {
				// 	fname = ""
				// }
				// zlog.Info("struct:", item.Kind, fieldName, "/", f.Type, f.Name, f.Type.PkgPath(), tag) //, c.Tag, c.Value, c.Value.Interface(), fval.CanAddr())
				l := level
				if !f.Anonymous {
					l++
				}
				c, e := itterate(l, fname, tname, tag, f.Anonymous, unnestAnonymous, recursive, fval)
				c.Address = fval.Addr().Interface()
				if e != nil {
					err = e
				} else {
					if c.Value.CanAddr() {
						c.Address = c.Value.Addr().Interface()
						//							zlog.Info("Addr:", c.Value, c.Address)
					}
					if unnestAnonymous && f.Anonymous {
						// zlog.Info("add anon", i, len(item.Children), len(c.Children))
						item.Children = append(item.Children, c.Children...)
					} else {
						item.Children = append(item.Children, c)
					}
				}
			}
		}

	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8:
		item.Kind = KindInt
		item.SimpleInterface = val.Int()
		item.Interface = val.Interface()

	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		item.Kind = KindInt
		item.SimpleInterface = val.Uint()
		item.Interface = val.Interface()

	case reflect.String:
		item.Kind = KindString
		item.Interface = val.Interface()

	case reflect.Bool:
		item.Kind = KindBool
		item.SimpleInterface = val.Bool()
		item.Interface = val.Interface()

	case reflect.Float32, reflect.Float64:
		item.Kind = KindFloat
		item.SimpleInterface = val.Float()
		item.Interface = val.Interface()

	case reflect.Map:
		item.Kind = KindMap
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()

	case reflect.Func:
		item.Kind = KindFunc
		item.SimpleInterface = val.Interface()
		item.Interface = val.Interface()

	default:
		zlog.Info("marshal.marshalValue: Carefull, unknown type!", val.Kind())
		item.Kind = KindUndef
	}
	//item.FieldName = fieldName
	item.IsAnonymous = isAnonymous
	item.Tag = tagName
	item.Value = val
	return
}

func ItterateStruct(istruct interface{}, unnestAnonymous, recursive bool) (item Item, err error) {
	rval := reflect.ValueOf(istruct)
	if !rval.IsValid() || rval.IsZero() {
		return
	}
	zlog.Assert(rval.Kind() == reflect.Ptr, "not pointer", rval.Kind(), rval)
	return itterate(0, "", "", "", false, unnestAnonymous, recursive, rval.Elem())
}
