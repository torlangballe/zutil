package zbool

import (
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

type BitSetStringer interface {
	FromStringToBits(str string)
	String() string
}

type BitsetItem struct {
	Name string
	Mask int64
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
