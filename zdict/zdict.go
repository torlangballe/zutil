package zdict

import (
	"fmt"
	"net/url"
	"reflect"
	"sort"

	"github.com/torlangballe/zutil/zint"

	"github.com/torlangballe/zutil/zfloat"

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
	unnestAnon := true
	recursive := false
	rootItems, err := zreflect.ItterateStruct(structure, unnestAnon, recursive)
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
	unnestAnon := true
	recursive := false
	rootItems, err := zreflect.ItterateStruct(structPtr, unnestAnon, recursive)
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
				zlog.AssertNotErr(err, item.FieldName, reflect.ValueOf(val).Kind())
				item.Value.Addr().Elem().SetFloat(f)
			case zreflect.KindInt:
				n, err := zint.GetAny(val)
				zlog.AssertNotErr(err)
				item.Value.Addr().Elem().SetInt(n)
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

func (d Items) FindName(name string) *Item {
	for i := range d {
		if d[i].Name == name {
			return &d[i]
		}
	}
	return nil
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

// GetItem get's an item at index i. Used to be compliant with MenuItem interface
func (d Items) GetItem(i int) (id, name string, value interface{}) {
	if i >= len(d) {
		return "", "", nil
	}
	return fmt.Sprint(d[i].Value), d[i].Name, d[i].Value
}

func (d Items) Count() int {
	return len(d)
}
