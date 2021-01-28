package zbool

import (
	"reflect"
	"strings"
	"sync/atomic"
)

// BoolInd is a bool which also has an indeterminate, or unknown  state
type BoolInd int

const (
	True    BoolInd = 1
	False           = 0
	Unknown         = -1
)

func IS(_ string) bool {
	return true
}

func NOT(_ string) bool {
	return false
}

func StringFor(val bool, strue, sfalse string) string {
	if val {
		return strue
	}
	return sfalse
}

func ToBoolInd(b bool) BoolInd {
	if b {
		return True
	}
	return False
}

func (b BoolInd) BoolValue() bool {
	return b == 1
}

func (b BoolInd) IsUndetermined() bool {
	return b == -1
}

func ChangeBit(all *int64, mask int64, on bool) {
	if on {
		*all |= mask
	} else {
		*all &= ^mask
	}
}

func FromString(str string, def bool) bool {
	bind := FromStringWithInd(str, Unknown)
	if bind == Unknown {
		return def
	}
	return bind.BoolValue()
}

func FromStringWithInd(str string, def BoolInd) BoolInd {
	if str == "-1" {
		return Unknown
	}
	if str == "1" || str == "true" || str == "TRUE" {
		return True
	}
	if str == "0" || str == "false" || str == "FALSE" {
		return False
	}
	return def
}

func ToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
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

type Atomic struct {
	flag int32
}

func (b *Atomic) Set(value bool) {
	var i int32 = 0
	if value {
		i = 1
	}
	atomic.StoreInt32(&(b.flag), int32(i))
}

func (b *Atomic) Get() bool {
	if atomic.LoadInt32(&(b.flag)) != 0 {
		return true
	}
	return false
}

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
