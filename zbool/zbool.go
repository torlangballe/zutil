package zbool

import (
	"strings"
)

// BoolInd is a bool which also has an indeterminate, or unknown  state
type BoolInd int

const (
	True    BoolInd = 1
	False           = 0
	Unknown         = -1
)

func ToBoolInd(b bool) BoolInd {
	if b {
		return True
	}
	return False
}

func (b BoolInd) Value() bool {
	return b == 1
}

func (b BoolInd) IsUndetermined() bool {
	return b == -1
}

func FromString(str string, def bool) bool {
	if str == "1" || str == "true" || str == "TRUE" {
		return true
	}
	if str == "0" || str == "false" || str == "FALSE" {
		return false
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
				str += "|"
			}
			str += list[i].Name
			// zlog.Info("parts:", list[i].Name, n, m)
			n &= ^m
		}
	}
	return str
}
