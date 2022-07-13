package zbits

import (
	"reflect"
	"strings"
)

type BitsetItem struct {
	Name  string
	Mask  int64
	Title string
}

type BitsetItemsOwner interface {
	GetBitsetItems() []BitsetItem
}

func BSItem(name string, mask int64) BitsetItem {
	return BitsetItem{Name: name, Mask: mask}
}

func BSItemTitled(name string, mask int64, title string) BitsetItem {
	return BitsetItem{Name: name, Mask: mask, Title: title}
}

func BitSetOwnerToString(b interface{}) string {
	bso := b.(BitsetItemsOwner)
	bitset := bso.GetBitsetItems()
	n := reflect.ValueOf(b).Int()
	return Int64ToStringFromList(int64(n), bitset)
}

func BitSetOwnerUnmarshal(ptr interface{}, data []byte) {
	val := reflect.ValueOf(ptr)
	eval := val.Elem()
	ei := eval.Interface()
	bso := ei.(BitsetItemsOwner)
	bitset := bso.GetBitsetItems()
	n := StrToInt64FromList(strings.Trim(string(data), `"`), bitset)
	eval.SetInt(int64(n))
}

func BitSetOwnerMarshal(b interface{}) ([]byte, error) {
	str := BitSetOwnerToString(b)
	return []byte(`"` + str + `"`), nil
}

func BitSetOwnerTitle(b interface{}) string {
	bso := b.(BitsetItemsOwner)
	bitset := bso.GetBitsetItems()
	n := reflect.ValueOf(b).Int()
	for _, bs := range bitset {
		if bs.Mask == int64(n) {
			return bs.Title
		}
	}
	return ""
}

func StrToInt64FromList(str string, list []BitsetItem) (n int64) {
	// zlog.Info("checkxxx:", list)
	for _, p := range strings.Split(str, "|") {
		for _, b := range list {
			if b.Name == p {
				n |= b.Mask
				break
			}
		}
	}
	return
}

func Int64ToStringFromList(n int64, list []BitsetItem) string {
	var str string
	for i := len(list) - 1; i >= 0; i-- {
		m := list[i].Mask
		// zlog.Info("check:", i, n, m, list[i].Name)
		if n&m == m {
			if str != "" {
				if m == 0 { // don't add 0-mask name if others
					continue
				}
				str += "|"
			}
			str += list[i].Name
			// zlog.Info("parts:", list[i].Name, n, m)
			n &= ^m
		}
	}
	return str
}
