package zdict

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/torlangballe/zutil/zbits"
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

func BitsetToItem(bi zbits.BitsetItem) Item {
	return Item{Name: bi.TitleOrName(), Value: bi.Mask}
}

func (item Item) Equal(to Item) bool {
	if item.Name != to.Name {
		return false
	}
	if item.Value == nil && to.Value == nil {
		return true
	}
	if item.Value == nil || to.Value == nil {
		return false
	}
	return reflect.DeepEqual(item.Value, to.Value)
}

func (items Items) Equal(to Items) bool {
	if len(items) != len(to) {
		return false
	}
	for i := range items {
		if !items[i].Equal(to[i]) {
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

func (d Dict) AsString(assigner, separator string) string {
	var all []string
	for k, v := range d {
		all = append(all, fmt.Sprint(k, assigner, v))
	}
	return strings.Join(all, separator)
}

func FromString(str, assigner, separator string) Dict {
	d := Dict{}
	for _, part := range strings.Split(str, separator) {
		var key, val string
		if zstr.SplitN(part, assigner, &key, &val) {
			d[key] = val
		}
	}
	return d
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

func (d Dict) ToStruct(structPtr any) {
	zreflect.ForEachField(structPtr, true, func(index int, fval reflect.Value, sf reflect.StructField) bool {
		dtags := zreflect.GetTagAsMap(string(sf.Tag))["zdict"]
		name := sf.Name
		hasTag := (len(dtags) != 0)
		if hasTag {
			name = dtags[0]
		}
		val, got := d[name]
		if !got && !hasTag {
			name = zstr.FirstToTitleCase(name)
			val, got = d[name]
		}
		// zlog.Info("Dict2Struct1:", name, fval.Kind())
		switch fval.Kind() {
		case reflect.String:
			str, got := val.(string)
			zlog.Assert(got)
			fval.Addr().Elem().SetString(str)
		case reflect.Float32, reflect.Float64:
			f, err := zfloat.GetAny(val)
			zlog.AssertNotError(err, name, fval.Kind())
			fval.Addr().Elem().SetFloat(f)
		case reflect.Int:
			n, err := zint.GetAny(val)
			zlog.AssertNotError(err)
			fval.Addr().Elem().SetInt(n)
		case reflect.Bool:
			b, isBool := val.(bool)
			if !isBool {
				str, _ := val.(string)
				if str != "" {
					b = zbool.FromString(str, false)
				}
			}
			fval.Addr().Elem().SetBool(b)
		case reflect.Map:
			_, got1 := fval.Interface().(map[string]string)
			_, got2 := val.(map[string]string)
			// zlog.Info("Got1&2", sdict, ddict, got1, got2)
			if got1 && got2 {
				fval.Set(reflect.ValueOf(val))
			}
		}
		return true
		// zlog.Info("Dict2Struct:", name, val, fval.Interface())
	})

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
		m, _ := v.(map[string]string)
		if m != nil {
			str = zstr.ArgsToString(m, ",", "=", "")
		}
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
	// zlog.Info("JSONStringInterfaceMap scan", val)
	if val == nil {
		*d = Dict{}
		return nil
	}
	data, ok := val.([]byte)
	if !ok {
		str, ok := val.(string)
		if ok {
			data = []byte(str)
		} else {
			return zlog.NewError("zdict.Dict Scan unsupported data type", reflect.TypeOf(val), reflect.ValueOf(val).Kind())
		}
	}
	// zlog.Info("JSONStringInterfaceMap scan2", string(data))
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
