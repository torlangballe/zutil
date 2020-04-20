package zbool

// BoolInd is a bool which also has an indeterminate, or unknown  state
type BoolInd int

const (
	BoolTrue    BoolInd = 1
	BoolFalse           = 0
	BoolUnknown         = -1
)

func ToBoolInd(b bool) BoolInd {
	if b {
		return BoolTrue
	}
	return BoolFalse
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
