package zdict

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"sort"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type Dict map[string]interface{}

type Item struct {
	Name  string
	Value interface{}
}

type Items []Item

type ItemsGetter interface {
	GetItems() Items
}

func (items Items) Equal(to Items) bool {
	if len(items) != len(to) {
		return false
	}
	for i := range items {
		if items[i] != to[i] {
			return false
		}
	}
	return true
}

func (i *Items) Empty() {
	*i = Items{}
}

func (d Dict) Join(equal, sep string) string {
	str := ""
	for k, v := range d {
		if str != "" {
			str += sep
		}
		str += fmt.Sprint(k, equal, v)
	}
	return str
}

func (d Dict) Copy() Dict {
	out := Dict{}
	for k, v := range d {
		sub, got := v.(map[string]interface{})
		if got {
			out[k] = Dict(sub).Copy()
			continue
		}
		dsub, got := v.(Dict)
		if got {
			out[k] = dsub.Copy()
			continue
		}
		out[k] = v
	}
	return out
}

func (d Dict) SortedKeys() (keys []string) {
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return
}

func (d Dict) Values() []interface{} {
	values := make([]interface{}, len(d), len(d))
	i := 0
	for _, v := range d {
		values[i] = v
		i++
	}
	return values
}

func (d Dict) RemoveAll() {
	for k := range d {
		delete(d, k)
	}
}

func (d Dict) AsURLParameters() string {
	return d.ToURLValues().Encode()
}

func FromStruct(structure interface{}, lowerFirst bool) Dict {
	d := Dict{}
	options := zreflect.Options{UnnestAnonymous: true, Recursive: true}
	rootItems, err := zreflect.ItterateStruct(structure, options)
	if err != nil {
		panic(err)
	}
	for _, item := range rootItems.Children {
		name := item.FieldName
		if lowerFirst {
			name = zstr.FirstToLower(name)
		}
		d[name] = item.Interface
	}
	return d
}

func (d Dict) ToStruct(structPtr interface{}) {
	options := zreflect.Options{UnnestAnonymous: true, Recursive: true}
	rootItems, err := zreflect.ItterateStruct(structPtr, options)
	if err != nil {
		panic(err)
	}
	for _, item := range rootItems.Children {
		var val interface{}
		name := item.FieldName
		vals := zreflect.GetTagAsMap(item.Tag)["zdict"]
		hasTag := (len(vals) != 0)
		if hasTag {
			name = vals[0]
		}
		val, got := d[name]
		if !got && !hasTag {
			val, got = d[zstr.FirstToTitleCase(name)]
		}
		if got {
			switch item.Kind {
			case zreflect.KindString:
				str, got := val.(string)
				zlog.Assert(got)
				item.Value.Addr().Elem().SetString(str)
			case zreflect.KindFloat:
				f, err := zfloat.GetAny(val)
				zlog.AssertNotError(err, item.FieldName, reflect.ValueOf(val).Kind())
				item.Value.Addr().Elem().SetFloat(f)
			case zreflect.KindInt:
				n, err := zint.GetAny(val)
				zlog.AssertNotError(err)
				item.Value.Addr().Elem().SetInt(n)
			case zreflect.KindBool:
				b, isBool := val.(bool)
				if !isBool {
					str, _ := val.(string)
					if str != "" {
						b = zbool.FromString(str, false)
					}
				}
				item.Value.Addr().Elem().SetBool(b)
			}
		}
	}
}

func FromURLValues(values url.Values) Dict {
	m := Dict{}
	for k, v := range values {
		m[k] = v[0]
	}
	return m
}

func (d Dict) ToURLValues() url.Values {
	vals := url.Values{}
	for k, v := range d {
		str := fmt.Sprint(v)
		vals.Add(k, str)
	}
	return vals
}

func (d Dict) URL(prefix string) string {
	return prefix + "?" + d.AsURLParameters()
}

func (d Dict) Dump() {
	zlog.Debug("zdict dump")
	for k, v := range d {
		fmt.Print(k, ": '", v, "' ", reflect.ValueOf(v).Kind(), reflect.ValueOf(v).Type(), "\n")
	}
}

func (d Dict) Value() (driver.Value, error) {
	// zlog.Info("JSONStringInterfaceMap Value")
	if d == nil {
		return nil, nil
	}
	return json.Marshal(d)
}

func (d *Dict) Scan(val interface{}) error {
	//	zlog.Info("JSONStringInterfaceMap scan")
	if val == nil {
		*d = Dict{}
		return nil
	}
	data, ok := val.([]byte)
	if !ok {
		return errors.New("zdict.Dict Scan unsupported data type")
	}
	return json.Unmarshal(data, d)
}

func (d Items) FindName(name string) *Item {
	for i := range d {
		if d[i].Name == name {
			return &d[i]
		}
	}
	return nil
}

func (d Items) FindValue(v interface{}) *Item {
	var empty *Item
	for i, di := range d {
		// zlog.Info("FV:", di.Value, "==", v, reflect.DeepEqual(di.Value, v), di.Value == nil, v == nil, reflect.ValueOf(v).Kind(), reflect.ValueOf(v).Type())
		if reflect.DeepEqual(di.Value, v) {
			// zlog.Info("zdict.Items FindValue: Found value for", v, i)
			return &d[i]
		}
		if di.Name == "" {
			empty = &d[i]
		}
	}
	// if empty == nil {
	// 	zlog.Info("zdict.Items FindValue: No value for", v, len(d))
	// }
	return empty
}

func (d *Items) RemoveAll() {
	*d = (*d)[:0]
}

func (d *Items) Add(name string, value interface{}) {
	*d = append(*d, Item{name, value})
}

func (d *Items) AddAtStart(name string, value interface{}) {
	*d = append([]Item{Item{name, value}}, (*d)...)
}

// GetItem get's an item at index i. Used to be compliant with NamedValues interface
func (d Items) GetItem(i int) (id, name string, value interface{}) {
	if i >= len(d) {
		return "", "", nil
	}
	return fmt.Sprint(d[i].Value), d[i].Name, d[i].Value
}

func (d Items) Count() int {
	return len(d)
}

type NamedValues interface {
	GetItem(i int) (id, name string, value interface{})
	Count() int
}

// NVStringer implements a method to create an id for using a type as an item in NamedValues
type NVStringer interface {
	ZNVID() string
}

// MenuItemsIndexOfID loops through items and returns index of one with id. -1 if none
func NamedValuesIndexOfID(m NamedValues, findID string) int {
	for i := 0; i < m.Count(); i++ {
		id, _, _ := m.GetItem(i)
		if findID == id {
			return i
		}
	}
	return -1
}

func NamedValuesIDForValue(m NamedValues, val interface{}) string {
	for i := 0; i < m.Count(); i++ {
		id, _, v := m.GetItem(i)
		if reflect.DeepEqual(val, v) {
			return id
		}
	}
	return ""
}

func NamedValuesAreEqual(a, b NamedValues) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	ac := a.Count()
	bc := b.Count()
	if ac != bc {
		return false
	}
	for i := 0; i < ac; i++ {
		ai, an, av := a.GetItem(i)
		bi, bn, bv := b.GetItem(i)
		if ai != bi {
			return false
		}
		if an != bn {
			return false
		}
		if !reflect.DeepEqual(av, bv) {
			return false
		}
	}
	return true
}

func DumpNamedValues(nv NamedValues) {
	c := nv.Count()
	for i := 0; i < c; i++ {
		id, name, _ := nv.GetItem(i)
		zlog.Info("Item:", id, name)
	}
}

func GetAnyStringKeyFromMap(m interface{}) string {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return ""
	}
	str, _ := keys[0].Interface().(string)
	return str
}

func GetAnyValueFromMap(getPtr interface{}, m interface{}) error {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return errors.New("no items")
	}
	v := mval.MapIndex(keys[0])
	reflect.ValueOf(getPtr).Elem().Set(v)
	return nil
}
