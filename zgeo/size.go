package zgeo

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type Size struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type Sizes []Size

const UndefValue = math.MaxFloat32

var (
	SizeUndef = Size{W: UndefValue, H: UndefValue}
	SizeNull  Size
)

// SizeF creates a Size from float64 w and h
func SizeF(w, h float32) Size {
	return Size{float64(w), float64(h)}
}

// SizeF creates a Size from float64 w and h
func SizeD(w, h float64) Size {
	return Size{w, h}
}

// SizeI creates a Size from integer w and h
func SizeI(w, h int) Size {
	return Size{float64(w), float64(h)}
}

// SizeI64 creates a Size from int64 w and h
func SizeI64(w, h int64) Size {
	return Size{float64(w), float64(h)}
}

// SizeBoth uses a for W and H
func SizeBoth(a float64) Size {
	return Size{a, a}
}

// Pos converts a size to a Pos
func (s Size) Pos() Pos {
	return Pos{s.W, s.H}
}

// IsNull returns true if S and W are zero
func (s Size) IsNull() bool {
	return s.W == 0 && s.H == 0
}

func (s Size) IsUndef() bool {
	return s.W == math.MaxFloat32
}

func (s *Size) Set(w, h float64) {
	s.W = w
	s.H = h
}

// Vertice returns the non-vertical s.W or vertical s.H
func (s Size) Vertice(vertical bool) float64 {
	if vertical {
		return s.H
	}
	return s.W
}

// VerticeP returns a pointer to the non-vertical s.W or vertical s.H
func (s *Size) VerticeP(vertical bool) *float64 {
	if vertical {
		return &s.H
	}
	return &s.W
}

// Max returns the greater of W and H
func (s Size) Max() float64 {
	return math.Max(s.W, s.H)
}

// Min returns the lesser of W and H
func (s Size) Min() float64 {
	return math.Min(s.W, s.H)
}

func (s Size) MaxWith(a Size) Size {
	w := math.Max(s.W, a.W)
	h := math.Max(s.H, a.H)
	return Size{w, h}
}

func (s Size) MinWith(a Size) Size {
	w := math.Min(s.W, a.W)
	h := math.Min(s.H, a.H)
	return Size{w, h}
}

// EqualSided returns a Size where W and H are largest of the two
func (s Size) EqualSided() Size {
	m := s.Max()
	return Size{m, m}
}

// Area returns the product of W, H (WxH)
func (s Size) Area() float64 {
	if s.W < 0 || s.H < 0 {
		return 0
	}
	return s.W * s.H
}

func (s *Size) Maximize(a Size) {
	if s.W == UndefValue {
		s.W = a.W
	} else {
		s.W = math.Max(s.W, a.W)
	}
	if s.H == UndefValue {
		s.H = a.W
	} else {
		s.H = math.Max(s.H, a.H)
	}
}

func (s *Size) Minimize(a Size) {
	if s.W == UndefValue {
		s.W = a.W
	} else {
		s.W = math.Min(s.W, a.W)
	}
	if s.H == UndefValue {
		s.H = a.W
	} else {
		s.H = math.Min(s.H, a.H)
	}
}

func (s *Size) MinimizeNonZero(a Size) {
	if a.W != 0 {
		if s.W == UndefValue {
			s.W = a.W
		} else {
			s.W = math.Min(s.W, a.W)
		}
	}
	if a.H != 0 {
		if s.H == UndefValue {
			s.H = a.W
		} else {
			s.H = math.Min(s.H, a.H)
		}
	}
}

func (s *Size) Add(a Size) {
	s.W += a.W
	s.H += a.H
}

func (s *Size) MultiplyD(a float64) {
	s.W *= a
	s.H *= a
}

func (s *Size) DivideD(a float64) {
	s.W /= a
	s.H /= a
}

func (s *Size) MultiplyF(a float32) {
	s.W *= float64(a)
	s.H *= float64(a)
}

func (s Size) Negative() Size {
	return Size{-s.W, -s.H}
}

func (s Size) Equals(a Size) bool {
	return s.W == a.W && s.H == a.H
}

func (s Size) Contains(a Size) bool {
	return s.W >= a.W && s.H >= a.H
}

func (s Size) Plus(a Size) Size             { return Size{s.W + a.W, s.H + a.H} }
func (s Size) Minus(a Size) Size            { return Size{s.W - a.W, s.H - a.H} }
func (s Size) MinusD(a float64) Size        { return Size{s.W - a, s.H - a} }
func (s Size) Times(a Size) Size            { return Size{s.W * a.W, s.H * a.H} }
func (s Size) TimesD(a float64) Size        { return Size{s.W * a, s.H * a} }
func (s Size) DividedBy(a Size) Size        { return Size{s.W / a.W, s.H / a.H} }
func (s Size) DividedByD(a float64) Size    { return Size{s.W / a, s.H / a} }
func (s Size) MultipliedBy(a Size) Size     { return Size{s.W * a.W, s.H * a.H} }
func (s Size) MultipliedByD(a float64) Size { return Size{s.W * a, s.H * a} }
func (s *Size) Subtract(a Size)             { s.W -= a.W; s.H -= a.H }
func (s *Size) SubtractD(d float64)         { s.W -= d; s.H -= d }

func (s Size) Copy() Size {
	return s
}

// ShrunkInto makes s just fit in 'in' proportionally, if bigger
func (s Size) ShrunkInto(in Size) Size {
	if s.W <= in.W && s.H < in.H {
		return s
	}
	return s.ScaledTo(in)
}

// ExpandTo makes s just fit in 'in' proportionally, if smaller
func (s Size) ExpandedInto(in Size) Size {
	if s.W >= in.W && s.H > in.H {
		return s
	}
	return s.ScaledTo(in)
}

// ScaledTo makes s just fit in 'in' proportionally
func (s Size) ScaledTo(in Size) Size {
	if s.W == 0 || s.H == 0 || in.W == 0 || in.H == 0 {
		return Size{}
	}
	f := in.DividedBy(s)
	min := f.Min()
	scaled := s.TimesD(min)
	return scaled
}

func (s Size) Floor() Size {
	return Size{math.Floor(s.W), math.Floor(s.H)}
}

func (s Size) Ceil() Size {
	return Size{math.Ceil(s.W), math.Ceil(s.H)}
}

func (s Size) Round() Size {
	return Size{math.Round(s.W), math.Round(s.H)}
}

func (s Size) String() string { // we don't use String() since we're doing that as set methods in zui
	return fmt.Sprintf("%gx%g", s.W, s.H)
}

func SizeFromString(str string) (Size, error) { // we don't use String() since that's special in Go
	var sw, sh string
	if !zstr.SplitN(str, "x", &sw, &sh) {
		return Size{}, errors.New("no x parsing zgeo.Size string: " + str)
	}
	w, err := strconv.ParseFloat(sw, 64)
	if err != nil {
		return Size{}, zlog.Error(err, zlog.StackAdjust(1), "parse w", sw)
	}
	h, err := strconv.ParseFloat(sh, 64)
	if err != nil {
		return Size{}, zlog.Error(err, zlog.StackAdjust(1), "parse h", sh)
	}
	return Size{w, h}, nil
}

func (s Size) ZUIString() string {
	return s.String()
}

func (s *Size) ZUISetFromString(str string) {
	*s, _ = SizeFromString(str)
}

func (s *Size) UnmarshalJSON(b []byte) error {
	var js struct {
		W float64 `json:"w"`
		H float64 `json:"h"`
	}
	// zlog.Info("Size.UnmarshalJSON")
	if bytes.ContainsAny(b, "{") {
		err := json.Unmarshal(b, &js)
		if err != nil {
			return err
		}
		*s = js
		return nil
	}
	str := string(b)
	str = strings.Trim(str, `"`)
	if len(str) == 0 {
		*s = Size{}
		return nil
	}
	var err error
	*s, err = SizeFromString(str)
	// fmt.Println("UNMARSHAL SIZE:", str, s)
	return err
}

func (s *Size) MarshalJSON() ([]byte, error) {
	if s.IsNull() {
		return []byte(`""`), nil
	}
	str := `"` + s.String() + `"`
	return []byte(str), nil
}

func (s Size) Value() (driver.Value, error) {
	return []byte(s.String()), nil
}

func (s *Size) Scan(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	size, err := SizeFromString(str)
	if err != nil {
		return err
	}
	*s = size
	return nil
}

func (s Sizes) GetItems() (items zdict.Items) {
	for _, size := range s {
		items = append(items, zdict.Item{Name: size.String(), Value: size})
	}
	return
}

func (s *Sizes) IndexOf(size Size) int {
	for i, is := range *s {
		if is == size {
			return i
		}
	}
	return -1
}

func (s Size) Swapped() Size {
	return Size{s.H, s.W}
}

type ISize struct {
	W int `json:"w"`
	H int `json:"h"`
}

type FSize struct {
	W float32 `json:"w"`
	H float32 `json:"h"`
}
