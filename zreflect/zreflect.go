package zreflect

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zstr"
)

// https://utcc.utoronto.ca/~cks/space/blog/programming/GoAddressableValues

var (
	timeType = reflect.TypeOf(time.Time{})
)

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
	KindArray              = "slice"
	KindInterface          = "interface"
	KindPointer            = "pointer"
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
	Address         any
	Package         string
	Value           reflect.Value
	Interface       any
	SimpleInterface any
	Children        []Item
}

type Options struct {
	UnnestAnonymous        bool
	Recursive              bool
	MakeSliceElementIfNone bool
}

func KindFromReflectKindAndType(kind reflect.Kind, rtype reflect.Type) TypeKind {
	switch kind {
	case reflect.Interface:
		return KindInterface
	case reflect.Ptr:
		return KindPointer
	case reflect.Slice:
		return KindSlice
	case reflect.Array:
		return KindArray
	case reflect.Struct:
		if rtype == timeType {
			return KindTime
		}
		return KindStruct
	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8, reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
		return KindInt
	case reflect.String:
		return KindString
	case reflect.Bool:
		return KindBool
	case reflect.Float32, reflect.Float64:
		return KindFloat
	case reflect.Map:
		return KindMap
	case reflect.Func:
		return KindFunc
	}
	return KindUndef
}

func itterate(level int, fieldName, typeName, tagName string, isAnonymous bool, val reflect.Value, options Options) (item Item, err error) {
	item.FieldName = fieldName
	vtype := val.Type()
	if typeName == "" {
		typeName = vtype.Name()
	}
	item.TypeName = typeName
	item.Package = vtype.PkgPath()
	if !val.CanInterface() {
		fmt.Println("zreflect.itterate: can't interface to:", fieldName)
		return
	}
	if !val.IsValid() {
		err = errors.New("marshalValue: val.IsValid() failed")
		return
	}
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

	case reflect.Slice:
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
		item.IsSlice = true
		item.Kind = KindSlice
		item.Interface = val.Interface()
		item.SimpleInterface = val.Interface()
		if options.Recursive { // !MakeSliceElementIfNone, wont happen if length is 0 anyway
			for i := 0; i < length; i++ {
				v := val.Index(i)
				sliceItem, serr := itterate(level+1, "", t.Name(), "", isAnonymous, v, options)
				if serr != nil {
					fmt.Println(serr, "slice item itterate")
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
				if !fval.CanInterface() {
					continue
				}
				tag := string(f.Tag) // http://golang.org/pkg/reflect/#StructTag
				tname := fval.Type().Name()
				fname := f.Name
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
					}
					if options.UnnestAnonymous && f.Anonymous {
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
		fmt.Println("marshal.marshalValue: Carefull, unknown type!", val.Kind())
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

func ItterateStruct(istruct any, options Options) (item Item, err error) {
	rval := reflect.ValueOf(istruct)
	if !rval.IsValid() { //|| rval.IsZero() { //  && rval.Kind() != reflect.StructKind
		fmt.Println("ItterateStruct: not valid", rval.IsValid(), rval.IsZero(), rval.Type(), rval.Kind())
		return
	}
	if rval.Kind() != reflect.Ptr {
		panic(zstr.Spaced("not pointer:", rval.Kind(), rval.Type(), rval))
	}
	return itterate(0, "", "", "", false, rval.Elem(), options)
}

type FieldInfo struct {
	FieldIndex   int // this is the index of the field _used_ field, not adding skipped, and increasing in anon
	ReflectValue reflect.Value
	StructField  reflect.StructField
}

func ForEachField(istruct any, flatten func(f reflect.StructField) bool, got func(each FieldInfo) bool) {
	forEachField(reflect.ValueOf(istruct), flatten, 0, got)
}

func FlattenIfAnonymous(f reflect.StructField) bool {
	return f.Anonymous
}

func FlattenAll(f reflect.StructField) bool {
	return true
}

func forEachField(rval reflect.Value, flatten func(f reflect.StructField) bool, istart int, got func(each FieldInfo) bool) (stoppedAt int, quit bool) {
	if rval.Kind() == reflect.Pointer {
		rval = rval.Elem()
	}
	if !rval.IsValid() {
		panic("forEachField: rval not valid")
	}
	if rval.Kind() == reflect.Slice || rval.Kind() == reflect.Map {
		return
	}
	j := istart
	for i := 0; i < rval.NumField(); i++ {
		fv := rval.Field(i)
		f := rval.Type().Field(i)
		if !fv.CanInterface() {
			j++
			continue
		}
		if fv.Kind() == reflect.Struct && flatten != nil && flatten(f) {
			var quit bool
			j, quit = forEachField(fv, flatten, j, got)
			if quit {
				return j, true
			}
			continue
		}
		if !got(FieldInfo{j, fv, f}) {
			return j, true
		}
		j++
	}
	return j, false
}

func FieldForIndex(istruct any, flatten func(f reflect.StructField) bool, index int) FieldInfo {
	var found FieldInfo
	found.FieldIndex = -1
	ForEachField(istruct, flatten, func(each FieldInfo) bool {
		if each.FieldIndex == index {
			found = each
			return false
		}
		return true
	})
	return found
}

func StructFieldNames(istruct any, flatten func(f reflect.StructField) bool) []string {
	var names []string
	ForEachField(istruct, flatten, func(each FieldInfo) bool {
		names = append(names, each.StructField.Name)
		return true
	})
	return names
}

func FieldForName(istruct any, flatten func(f reflect.StructField) bool, name string) (FieldInfo, bool) {
	var finfo FieldInfo
	var found bool
	ForEachField(istruct, flatten, func(each FieldInfo) bool {
		if each.StructField.Name == name {
			finfo = each
			found = true
			return false
		}
		return true
	})
	return finfo, found
}

// GetTagAsMap returns a map of label:[vars] `json:"id, omitempty"` -> json : [id, omitempty]
var tagRegEx, _ = regexp.Compile(`(\w+)\s*:"([^"]*)"\s*`) // http://regoio.herokuapp.com

func GetTagAsMap(stag string) map[string][]string {
	if stag != "" {
		m := map[string][]string{}
		matches := tagRegEx.FindAllStringSubmatch(stag, -1)
		if len(matches) > 0 {
			for _, groups := range matches {
				label := groups[1]
				g := groups[2]
				g = strings.Replace(g, "\\n", "\n", -1)
				vars := zstr.SplitStringWithDoubleAsEscape(g, ",")
				m[label] = vars
			}
			return m
		}
	}
	return nil
}

func GetTagValuesForKey(stag reflect.StructTag, key string) (vals []string, ignore bool) {
	str, got := stag.Lookup(key)
	// fmt.Println("GetTagValuesForKey", str, got, stag, key)
	if !got {
		return nil, false
	}
	vals = zstr.SplitStringWithDoubleAsEscape(str, ",")
	if len(vals) == 1 && vals[0] == "-" {
		return nil, true
	}
	return vals, false
}

func DeepCopy(destPtr, source any) error {
	buf := bytes.Buffer{}
	err := gob.NewEncoder(&buf).Encode(source)
	if err != nil {
		return err
	}
	return gob.NewDecoder(&buf).Decode(destPtr)
}

// CopyVal makes a copy of rval. If rval is a pointer, it makes a copy of element and returns a pointer
func CopyAny(v any) any {
	rval := reflect.ValueOf(v)
	isPointer := (rval.Kind() == reflect.Pointer)
	if isPointer {
		rval = rval.Elem()
	}
	n := reflect.New(rval.Type()).Elem()
	n.Set(rval)
	if isPointer {
		n = n.Addr()
	}
	return n.Interface()
}

// NewOfAny returns a new'ed item of that type.
// If a is a pointer, its element is used.
func NewOfAny(a any) any {
	val := reflect.ValueOf(a)
	if val.Kind() == reflect.Pointer {
		val = val.Elem()
	}
	return reflect.New(val.Type()).Interface()
}

func IsRealSlice(val reflect.Value) bool {
	var f []float32
	if val.Type() == reflect.TypeOf(f) {
		return true
	}
	var d []float64
	return val.Type() == reflect.TypeOf(d)
}

// AnySetWithRelaxedNumbers sets int, float values from->to
func AnySetWithRelaxedNumbers(to, from reflect.Value) {
	kfrom := KindFromReflectKindAndType(from.Kind(), from.Type())
	kto := KindFromReflectKindAndType(to.Kind(), to.Type())
	if kfrom == KindInt && kto == KindFloat {
		to.SetFloat(float64(from.Int()))
		return
	}
	if kfrom == KindFloat && kto == KindInt {
		to.SetInt(int64(from.Float()))
		return
	}
	to.Set(from)
}

func HashAnyToInt64(a interface{}, add string) int64 {
	str := fmt.Sprintf("%v", a) + add
	// fmt.Println("HashAnyToInt64", str)
	return zint.HashTo64(str)
}

func SetStringToAny(toPtr any, from string) {
	switch t := toPtr.(type) {
	case *bool:
		if from == "true" || from == "TRUE" || from == "1" {
			*t = true
		} else {
			*t = false
		}
	case *int, *int8, *int16, *int32, *int64, *uint, *uint8, *uint16, *uint32, *uint64:
		n, err := strconv.ParseInt(from, 10, 64)
		if err != nil {
			fmt.Println("SetAny1:", err)
		}
		zint.SetAny(toPtr, n)
	case *float32, *float64:
		n, err := strconv.ParseFloat(from, 64)
		if err != nil {
			fmt.Println("SetAny2:", err, from)
		}
		zfloat.SetAny(toPtr, n)
	case *string:
		*t = from
	default:
		fmt.Println("bad type:", reflect.TypeOf(toPtr))
	}
}
