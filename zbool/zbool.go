package zbool

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"

	"github.com/torlangballe/zutil/zint"
)

// BoolInd is a bool which also has an indeterminate, or unknown  state
type BoolInd int

const (
	Unknown         = 0
	False   BoolInd = 1
	True    BoolInd = 2
)

func ToBoolInd(b bool) BoolInd {
	if b {
		return True
	}
	return False
}

func (b BoolInd) Bool() bool {
	return b == True
}

func (b BoolInd) String() string {
	if b.IsUnknown() {
		return "undef"
	}
	return ToString(b.Bool())
}

func (b BoolInd) IsTrue() bool {
	return b == True
}

func (b BoolInd) IsFalse() bool {
	return b == False
}

func (b BoolInd) IsUnknown() bool {
	return b == Unknown
}

func StringFor(val bool, strue, sfalse string) string {
	if val {
		return strue
	}
	return sfalse
}

func FromString(str string, def bool) bool {
	bind := FromStringWithInd(str, Unknown)
	if bind == Unknown {
		return def
	}
	return bind.Bool()
}

func FromStringWithError(str string) (bool, error) {
	bind := FromStringWithInd(str, Unknown)
	if bind == Unknown {
		return false, errors.New("bad type: " + str)
	}
	return bind.Bool(), nil
}

func (bi *BoolInd) FromBool(b bool) {
	if b {
		*bi = True
	} else {
		*bi = False
	}
}

func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func FromBool(b bool) BoolInd {
	if b {
		return True
	}
	return False
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

func FromAny(a any) bool {
	switch t := a.(type) {
	case bool:
		return t
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return t != 0
	case string:
		n, _ := strconv.ParseFloat(t, 64)
		return n != 0
	}
	return false
}

func SetAny(toPtr any, from bool) {
	switch t := toPtr.(type) {
	case *bool:
		*t = from
	case *int, *int8, *int16, *int32, *int64, *uint, *uint8, *uint16, *uint32, *uint64, *float32, *float64:
		v := 0.0
		if from {
			v = 1
		}
		zint.SetAny(toPtr, int64(v))
	case *string:
		*t = ToString(from)
	}
	fmt.Println("bad type:", reflect.TypeOf(toPtr))
}
