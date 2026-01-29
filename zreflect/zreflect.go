package zreflect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zstr"
)

// https://utcc.utoronto.ca/~cks/space/blog/programming/GoAddressableValues
// https://bitfieldconsulting.com/posts/constraints -- good about generics constraints

type FieldInfo struct {
	FieldIndex   int // this is the index of the field _used_ field, not adding skipped, and increasing in anon
	ReflectValue reflect.Value
	StructField  reflect.StructField
}

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

var (
	TimeType = reflect.TypeOf(time.Time{})
)

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
		if rtype == TimeType {
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

func (f *FieldInfo) TagKeyValuesForKey(key string) (vals []zstr.KeyValue, skip bool) {
	return TagKeyValuesForKeyInStructField(&f.StructField, key)
}

func TagKeyValuesForKeyInStructField(sf *reflect.StructField, key string) (vals []zstr.KeyValue, skip bool) {
	str := sf.Tag.Get(key)
	return TagKeyValuesFromString(str)
}

func TagKeyValuesFromString(str string) (vals []zstr.KeyValue, skip bool) {
	cvalues, skip := TagPartAsCommaValues(str)
	if skip {
		return nil, true
	}
	return TagCommaValuesAsKeyValues(cvalues), false
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

var tagRegEx, _ = regexp.Compile(`(\w+)\s*:"([^"]*)"\s*`) // http://regoio.herokuapp.com

// GetTagAsMap returns a map of label:[vars] `json:"id, omitempty"` -> json : [id, omitempty]
func GetTagAsMap(stag string) map[string][]string {
	if stag != "" {
		m := map[string][]string{}
		matches := tagRegEx.FindAllStringSubmatch(stag, -1)
		if len(matches) > 0 {
			for _, groups := range matches {
				label := groups[1]
				g := groups[2]
				g = strings.Replace(g, "\\n", "\n", -1)
				vars := zstr.SplitStringWithDoubleAsEscape(g, ",", -1)
				m[label] = vars
			}
			return m
		}
	}
	return nil
}

func TagValuesForKey(stag reflect.StructTag, key string) (vals []string, ignore bool) {
	str, got := stag.Lookup(key)
	// fmt.Println("GetTagValuesForKey", str, got, stag, key)
	if !got {
		return nil, false
	}
	return TagPartAsCommaValues(str)
}

func TagPartAsCommaValues(stag string) (vals []string, ignore bool) {
	vals = zstr.SplitStringWithDoubleAsEscape(stag, ",", -1)
	if len(vals) == 1 && vals[0] == "-" {
		return nil, true
	}
	return vals, false
}

func TagCommaValuesAsKeyValues(values []string) []zstr.KeyValue {
	var keyVals []zstr.KeyValue
	for _, value := range values {
		var key, val string
		parts := zstr.SplitStringWithDoubleAsEscape(value, ":", 2)
		if len(parts) == 2 {
			key = parts[0]
			val = parts[1]
		} else {
			key = value
		}
		var kv zstr.KeyValue
		kv.Key = strings.TrimSpace(key)
		kv.Value = strings.TrimSpace(val)
		keyVals = append(keyVals, kv)
	}
	return keyVals
}

func DeepCopy(destPtr, source any) error {
	dv := reflect.ValueOf(destPtr).Elem()
	sv := reflect.ValueOf(source)
	if dv.Kind() == reflect.Map && sv.Kind() == reflect.Map { // we do a special case for copying between map[string]<anything> to map[string]string using fmt.Sprintf
		dt := dv.Type()
		st := sv.Type()
		// fmt.Println("DeepCopy1: copy specical map", dv.Len(), dv.IsNil())
		if dt.Key().Kind() == reflect.String && st.Key().Kind() == reflect.String &&
			dt.Elem().Kind() == reflect.String && st.Elem().Kind() != reflect.String {
			// fmt.Println("DeepCopy: copy specical map", dv.Len(), dv.IsNil())
			if dv.IsNil() {
				t := reflect.MapOf(dt.Key(), dt.Elem())
				dv.Set(reflect.MakeMap(t))
			}
			for _, keyR := range sv.MapKeys() {
				str := fmt.Sprint(sv.MapIndex(keyR).Interface())
				// fmt.Println("DeepCopyN:", keyR.String(), str)
				dv.SetMapIndex(keyR, reflect.ValueOf(str))
			}
			return nil
		}
	}
	buf := bytes.Buffer{}
	err := json.NewEncoder(&buf).Encode(source)
	if err != nil {
		return err
	}
	err = json.NewDecoder(&buf).Decode(destPtr)
	return err
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

func Swap[A any](a, b *A) {
	t := *a
	*a = *b
	*b = t
}

func MapToStruct(m map[string]any, structPtr any) error {
	var outErr error
	ForEachField(structPtr, FlattenIfAnonymous, func(each FieldInfo) bool {
		dtags := GetTagAsMap(string(each.StructField.Tag))["zdict"]
		name := each.StructField.Name
		hasTag := (len(dtags) != 0)
		if hasTag {
			name = dtags[0]
		}
		val := m[name]
		if val == nil {
			return true
		}
		rVal := each.ReflectValue
		if rVal.Kind() == reflect.Pointer {
			rVal = rVal.Elem()
		}
		switch rVal.Kind() {
		case reflect.Struct, reflect.Map:
			dest := rVal.Addr().Interface()
			err := DeepCopy(dest, val)
			// fmt.Println("MapToStruct Map:", each.StructField.Name, err, reflect.TypeOf(dest))
			if err != nil {
				outErr = fmt.Errorf("%w %s %s", err, each.StructField.Name, name)
				return false
			}
		case reflect.String:
			str, got := val.(string)
			if !got {
				outErr = fmt.Errorf("Not string convertable %s %s", each.StructField.Name, name)
				return false
			}
			rVal.SetString(str)
		case reflect.Float32, reflect.Float64:
			f, err := zfloat.GetAny(val)
			if err != nil {
				outErr = fmt.Errorf("%w %s %s", err, each.StructField.Name, name)
				return false
			}
			rVal.Addr().Elem().SetFloat(f)
		case reflect.Int:
			n, err := zint.GetAny(val)
			if err != nil {
				outErr = fmt.Errorf("%w %s %s", err, each.StructField.Name, name)
				return false
			}
			rVal.Addr().Elem().SetInt(n)
		case reflect.Bool:
			b, isBool := val.(bool)
			if !isBool {
				str, _ := val.(string)
				if str != "" {
					b = zbool.FromString(str, false)
				}
			}
			rVal.Addr().Elem().SetBool(b)
		}
		return true
	})
	return outErr
}

func MarshalEnum[S comparable](from S, m map[string]S) ([]byte, error) {
	for k, v := range m {
		if v == from {
			return []byte(k), nil
		}
	}
	return nil, fmt.Errorf("No value: %v", from)
}

func UnmarshalEnum[S comparable](to *S, data []byte, m map[string]S) error {
	key := string(data)
	v, got := m[key]
	if !got {
		return fmt.Errorf("No value for key: %s", key)
	}
	*to = v
	return nil
}

func PasteRegisteredItemsFromClipboardString[P any](str string) (P, error) {
	var stype string
	var p P
	line, body := zstr.SplitInTwo(str, "\n")
	if !zstr.HasPrefix(line, "zcopyitem: ", &stype) {
		return p, fmt.Errorf("Paste buffer doesn't contain items to paste: %s", stype)
	}
	rtype := reflect.TypeOf(p)
	stype2 := MakeTypeNameWithPackage(rtype)
	if stype != stype2 {
		err := fmt.Errorf("Paste type is not the same as wanted receive type\n%s\n%s", stype, stype2)
		return p, err
	}
	err := json.Unmarshal([]byte(body), &p)
	if err != nil {
		err := fmt.Errorf("Couldn't unpack paste data")
		return p, err
	}
	return p, nil
}
