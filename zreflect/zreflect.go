package zreflect

import (
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zint"
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

type Item struct {
	Kind            TypeKind
	BitSize         int
	TypeName        string
	FieldName       string
	Tag             string
	IsAnonymous     bool
	IsSlice         bool
	IsPointer       bool
	Address         interface{}
	Package         string
	Value           reflect.Value
	Interface       interface{}
	SimpleInterface interface{}
	Children        []Item
}

type Options struct {
	UnnestAnonymous        bool
	Recursive              bool
	MakeSliceElementIfNone bool
}

func itterate(level int, fieldName, typeName, tagName string, isAnonymous bool, val reflect.Value, options Options) (item Item, err error) {
	// zlog.Info("itterate:", level, fieldName, typeName, tagName)
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
	// zlog.Info("zref.Itterate:", fieldName, val.Kind())
	switch val.Kind() {
	case reflect.Interface:
		item.Interface = val.Interface()
		item.Kind = KindInterface

	case reflect.Ptr:
		t := vtype
		var v reflect.Value
		if val.IsNil() {
			t = vtype.Elem()
			v = reflect.Zero(t)
		} else {
			v = reflect.Indirect(val)
		}
		pItem, perr := itterate(level, fieldName, t.Name(), tagName, isAnonymous, v, options)
		err = perr
		item.Interface = val.Interface()
		item.IsPointer = true
		item.Kind = pItem.Kind
		item.Children = pItem.Children
		// zlog.Info("ptr:", t.Name(), fieldName, t.PkgPath(), item.IsPointer)
		//		item.Address = val.Addr().Interface()

	case reflect.Slice:
		// zlog.Info("slice1:", val.Len(), fieldName)
		t := reflect.TypeOf(val.Interface()).Elem()
		v := reflect.Zero(t)
		length := val.Len()
		if !v.CanAddr() {
			v = reflect.New(t)
		}
		item, err = itterate(level, fieldName, t.Name(), tagName, isAnonymous, v, options)
		if length == 0 && !options.MakeSliceElementIfNone {
			item.Children = item.Children[:0]
		}
		item.Value = val
		// zlog.Info("slice:", item.Value.Len(), t.Name(), fieldName, t.PkgPath(), v.Kind())
		item.IsSlice = true
		item.Kind = KindSlice
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()
		if options.Recursive { // !MakeSliceElementIfNone, wont happen if length is 0 anyway
			for i := 0; i < length; i++ {
				v := val.Index(i)
				sliceItem, serr := itterate(level+1, "", t.Name(), "", isAnonymous, v, options)
				if serr != nil {
					zlog.Error(serr, "slice item itterate")
					continue
				}
				item.Children = append(item.Children, sliceItem)
			}
		}
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
			// fmt.Printf("struct: %s %+v\n", fieldName, val.Interface())
			if level > 0 && !options.Recursive && (!options.UnnestAnonymous || !isAnonymous) { // always go into first level
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
				c, e := itterate(l, fname, tname, tag, f.Anonymous, fval, options)
				if fval.CanAddr() {
					c.Address = fval.Addr().Interface()
				}
				if e != nil {
					err = e
				} else {
					if c.Value.CanAddr() {
						c.Address = c.Value.Addr().Interface()
						//							zlog.Info("Addr:", c.Value, c.Address)
					}
					if options.UnnestAnonymous && f.Anonymous {
						// zlog.Info("add anon", i, len(item.Children), len(c.Children))
						item.Children = append(item.Children, c.Children...)
					} else {
						// if c.Kind == KindSlice {
						// 	zlog.Info("zreflect add slice child", i, c.FieldName, c.Interface, c.Value.Len, fval.Len())
						// }
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
	switch val.Kind() {
	case reflect.Int, reflect.Uint:
		item.BitSize = zint.SizeOfInt
	case reflect.Int16, reflect.Uint16:
		item.BitSize = 16
	case reflect.Int32, reflect.Uint32, reflect.Float32:
		item.BitSize = 32
	case reflect.Int64, reflect.Uint64, reflect.Float64:
		item.BitSize = 64
	case reflect.Int8, reflect.Uint8:
		item.BitSize = 8
	}
	//item.FieldName = fieldName
	item.IsAnonymous = isAnonymous
	item.Tag = tagName
	item.Value = val
	return
}

func ItterateStruct(istruct interface{}, options Options) (item Item, err error) {
	rval := reflect.ValueOf(istruct)
	if !rval.IsValid() || rval.IsZero() {
		zlog.Info("ItterateStruct: not valid")
		return
	}
	zlog.Assert(rval.Kind() == reflect.Ptr, "not pointer", rval.Kind(), rval)
	return itterate(0, "", "", "", false, rval.Elem(), options)
}

// GetTagAsFields returns a map of label:[vars] `json:"id, omitempty"` -> json : [id, omitempty]
var tagRegEx, _ = regexp.Compile(`(\w+)\s*:"([^"]*)"\s*`) // http://regoio.herokuapp.com

func GetTagAsMap(stag string) map[string][]string {
	if stag != "" {
		m := map[string][]string{}
		matches := tagRegEx.FindAllStringSubmatch(stag, -1)
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

func FindFieldWithNameInStruct(name string, structure interface{}, anonymous bool) (reflect.Value, bool) {
	val := reflect.ValueOf(structure)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	vtype := val.Type()
	n := vtype.NumField()
	for i := 0; i < n; i++ {
		f := vtype.Field(i)
		fval := val.Field(i)
		// zlog.Info("FindFieldWithNameInStruct:", name, f.Name)
		if f.Name == name {
			return fval, true
		}
		if anonymous && fval.Kind() == reflect.Struct && f.Anonymous {
			v, found := FindFieldWithNameInStruct(name, val.Field(i).Interface(), anonymous)
			if found {
				return v, true
			}
		}
	}
	return reflect.Value{}, false
}
