package zdict

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strings"

	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type Dict map[string]any

type Item struct {
	Name  string
	Value any
}

type ItemGetter interface {
	GetItem() Item
}

type Items []Item

type ItemsGetter interface {
	GetItems() Items
}

var AssertFunc func(success bool, parts ...any)

func ItemsFromRowGetterSlice(slice any) *Items {
	sval := reflect.ValueOf(slice)
	n := reflect.New(sval.Type().Elem())
	_, got := n.Interface().(ItemGetter)
	if !got {
		return nil
	}
	var items Items
	for i := 0; i < sval.Len(); i++ {
		g := sval.Index(i).Interface().(ItemGetter)
		item := g.GetItem()
		items = append(items, item)
	}
	return &items
}

func (item Item) GetName() string {
	return item.Name
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

func MakeItems(nameValPairs ...any) Items {
	var items Items

	for i := 0; i < len(nameValPairs); i += 2 {
		var di Item
		di.Name = fmt.Sprint(nameValPairs[i])
		di.Value = nameValPairs[i+1]
		items = append(items, di)
	}
	return items
}

func (d Dict) GetItems() Items {
	var items Items
	for k, v := range d {
		items = append(items, Item{Name: k, Value: v})
	}
	return items
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
		sub, got := v.(map[string]any)
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

func (d Dict) Keys() []string {
	var keys []string
	for k := range d {
		keys = append(keys, k)
	}
	return keys
}

func (d Dict) SortedKeys() []string {
	keys := d.Keys()
	sort.Strings(keys)
	return keys
}

func (d Dict) Values() []any {
	values := make([]any, len(d), len(d))
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

func (d Dict) AsShallowMap() map[string]any {
	m := map[string]any{}
	for k, v := range d {
		m[k] = v
	}
	return m
}

func (d *Dict) MergeIn(in Dict) {
	for k, v := range in {
		(*d)[k] = v
	}
}

func FromShallowMap(m map[string]any) Dict {
	d := Dict{}
	for k, v := range m {
		d[k] = v
	}
	return d
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

// ToStruct: Use zreflect.MapToStruct

func FromStruct(structure any, lowerFirst bool) Dict {
	d := Dict{}
	d.FillFromStruct(structure, lowerFirst)
	return d
}

func (d *Dict) FillFromStruct(structure any, lowerFirst bool) {
	zreflect.ForEachField(structure, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		dtags := zreflect.GetTagAsMap(string(each.StructField.Tag))["zdict"]
		name := each.StructField.Name
		if lowerFirst {
			name = zstr.FirstToLower(name)
		} else {
			hasTag := (len(dtags) != 0)
			if hasTag {
				name = dtags[0]
			}
		}
		(*d)[name] = each.ReflectValue.Interface()
		return true
	})
}

func FromURLValues(values url.Values) Dict {
	m := Dict{}
	for k, v := range values {
		if strings.HasPrefix(v[0], "{") && strings.HasSuffix(v[0], "}") {
			subMap := map[string]string{}
			err := json.Unmarshal([]byte(v[0]), &subMap)
			if err == nil {
				m[k] = subMap
				continue
			}
		}
		m[k] = v[0]
	}
	return m
}

func (d Dict) ToURLValues() url.Values {
	vals := url.Values{}
	for k, v := range d {
		var set bool
		var str string
		var err error
		m, is := v.(map[string]string)
		if is {
			if len(m) == 0 {
				continue
			}
			var data []byte
			data, err = json.Marshal(m)
			if err == nil {
				set = true
				str = string(data)
			} else {
				str = "<" + err.Error() + ">"
			}
			//			str = zstr.ArgsToString(m, ",", "=", "")
		}
		if !set && err == nil {
			str = fmt.Sprint(v)
		}
		vals.Add(k, str)
	}
	return vals
}

func (d Dict) URL(prefix string) string {
	return prefix + "?" + d.AsURLParameters()
}

func (d Dict) Dump() {
	fmt.Println("zdict dump")
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

func (d *Dict) Scan(val any) error {
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
			return fmt.Errorf("zdict.Dict Scan unsupported data type %v %v", reflect.TypeOf(val), reflect.ValueOf(val).Kind())
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

func (d Items) FindValue(v any) *Item {
	var empty *Item
	for i, di := range d {
		// fmt.Println("FV:", di.Value, "==", v, reflect.DeepEqual(di.Value, v), di.Value == nil, v == nil, reflect.TypeOf(di.Value), reflect.TypeOf(v))
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

func (d *Items) Add(name string, value any) {
	*d = append(*d, Item{name, value})
}

func (d *Items) AddAtStart(name string, value any) {
	*d = append([]Item{Item{name, value}}, (*d)...)
}

// GetItem get's an item at index i. Used to be compliant with NamedValues interface
func (d Items) GetItem(i int) (id, name string, value any) {
	if i >= len(d) {
		return "", "", nil
	}
	return fmt.Sprint(d[i].Value), d[i].Name, d[i].Value
}

func (d Items) Count() int {
	return len(d)
}

func (d Dict) WriteTabulated(w io.Writer) {
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 100

	for _, k := range d.SortedKeys() {
		v := d[k]
		fmt.Fprint(tabs, zstr.EscGreen, k, "\t", zstr.EscNoColor, v, "\n")
	}
	tabs.Flush()
}
