package zmap

import (
	"errors"
	"fmt"
	"reflect"
)

func GetAnyKeyAsString(m interface{}) string {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return ""
	}
	str, _ := keys[0].Interface().(string)
	return str
}

func GetAnyValue(getPtr interface{}, m interface{}) error {
	mval := reflect.ValueOf(m)
	keys := mval.MapKeys()
	if len(keys) == 0 {
		return errors.New("no items")
	}
	v := mval.MapIndex(keys[0])
	reflect.ValueOf(getPtr).Elem().Set(v)
	return nil
}

func GetKeysAsStrings(m interface{}) (keys []string) {
	mval := reflect.ValueOf(m)
	mkeys := mval.MapKeys()
	for i := 0; i < len(mkeys); i++ {
		k := fmt.Sprint(mkeys[i].Interface())
		keys = append(keys, k)
	}
	return
}
