package zbool

import "github.com/torlangballe/zutil/zlog"

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
		return false, zlog.NewError("bad type:", str)
	}
	return bind.Bool(), nil
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
