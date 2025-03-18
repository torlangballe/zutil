package zgeo

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"image"
	"math"
	"strconv"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zstr"
)

type Pos struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

var (
	PosUndef = PosD(zfloat.Undefined, zfloat.Undefined)
	PosNull  Pos
)

func PosF(x, y float32) Pos {
	return PosD(float64(x), float64(y))
}

func PosD(x, y float64) Pos {
	return Pos{X: x, Y: y}
}

func PosBoth(n float64) Pos {
	return PosD(n, n)
}

func (p Pos) String() string {
	return fmt.Sprintf("%g,%g", p.X, p.Y)
}

func PosFromString(str string) (Pos, error) {
	var sx, sy string
	if !zstr.SplitN(str, ",", &sx, &sy) {
		return Pos{}, errors.New("no ',' splitting zgeo.Pos string: " + str)
	}
	w, err := strconv.ParseFloat(sx, 64)
	if err != nil {
		return Pos{}, zlog.Error(zlog.StackAdjust(1), "parse x", sx, err)
	}
	h, err := strconv.ParseFloat(sx, 64)
	if err != nil {
		return Pos{}, zlog.Error(zlog.StackAdjust(1), "parse y", sy, err)
	}
	return PosD(w, h), nil
}

func (p Pos) ZUIString() string {
	return p.String()
}

func (p *Pos) ZUISetFromString(str string) {
	*p, _ = PosFromString(str)
}

func (p Pos) Element(vertical bool) float64 {
	if vertical {
		return p.Y
	}
	return p.X
}

func (p *Pos) SetOne(vertical bool, v float64) {
	if vertical {
		p.Y = v
	}
	p.X = v
}

func (p Pos) Size() Size {
	return Size{p.X, p.Y}
}

func (p *Pos) Set(x, y float64) {
	*p = Pos{x, y}
}

func (p *Pos) SetF(x, y float32) {
	*p = Pos{float64(x), float64(y)}
}

func (p *Pos) Swap() {
	*p = Pos{p.Y, p.X}
}

func (p Pos) Swapped() Pos {
	return Pos{p.Y, p.X}
}

func (p Pos) Max(a Pos) Pos {
	return Pos{math.Max(p.X, a.X), math.Max(p.Y, a.Y)}
}

func (p Pos) Min(a Pos) Pos {
	return Pos{math.Min(p.X, a.X), math.Min(p.Y, a.Y)}
}

func (p Pos) GetRot90CW() Pos    { return Pos{p.Y, -p.X} }
func (p Pos) Dot(a Pos) float64  { return p.X*a.X + p.Y*a.Y }
func (p Pos) Length() float64    { return math.Sqrt(p.X*p.X + p.Y*p.Y) }
func (p Pos) IsNull() bool       { return p.X == 0.0 && p.Y == 0.0 }
func (p Pos) GetNormalized() Pos { return p.DividedByD(p.Length()) }
func (p Pos) Sign() Pos          { return Pos{zmath.Sign(p.X), zmath.Sign(p.Y)} }
func (p Pos) Negative() Pos {
	return Pos{-p.X, -p.Y}
}
func (p Pos) Abs() Pos {
	return Pos{math.Abs(p.X), math.Abs(p.Y)}
}
func (p Pos) IsSameDirection(pos Pos) bool {
	if p == pos {
		return true
	}
	if zmath.Sign(pos.X) != zmath.Sign(p.X) || zmath.Sign(pos.Y) != zmath.Sign(p.Y) {
		return false
	}
	if pos.Y == 0.0 {
		return p.Y == 0.0
	}
	if p.Y == 0.0 {
		return p.Y == 0.0
	}
	if p.X/p.Y == pos.X/pos.Y {
		return true
	}
	return false
}

func (p Pos) RotatedCCW(angle float64) Pos {
	s := math.Sin(angle)
	c := math.Cos(angle)

	return Pos{p.X*c - p.Y*s, p.X*s + p.Y*c}
}

func (p *Pos) MultiplyD(a float64) {
	p.X *= a
	p.Y *= a
}

func (p Pos) GoPoint() image.Point {
	return image.Pt(int(p.X), int(p.Y))
}

func PosFromGoPoint(point image.Point) Pos {
	return Pos{float64(point.X), float64(point.Y)}
}

func (p Pos) PlusD(a float64) Pos      { return Pos{p.X + a, p.Y + a} }
func (p Pos) PlusX(x float64) Pos      { return Pos{p.X + x, p.Y} }
func (p Pos) PlusY(y float64) Pos      { return Pos{p.X, p.Y + y} }
func (p Pos) MinusD(a float64) Pos     { return Pos{p.X - a, p.Y - a} }
func (p Pos) TimesD(a float64) Pos     { return Pos{p.X * a, p.Y * a} }
func (p Pos) DividedByD(a float64) Pos { return Pos{p.X / a, p.Y / a} }
func (p Pos) Plus(a Pos) Pos           { return Pos{p.X + a.X, p.Y + a.Y} }
func (p Pos) Minus(a Pos) Pos          { return Pos{p.X - a.X, p.Y - a.Y} }
func (p Pos) Times(a Pos) Pos          { return Pos{p.X * a.X, p.Y * a.Y} }
func (p Pos) DividedBy(a Pos) Pos      { return Pos{p.X / a.X, p.Y / a.Y} }
func (p Pos) AddedSize(s Size) Pos     { return Pos{p.X + s.W, p.Y + s.H} }
func (p Pos) Equals(a Pos) bool        { return p.X == a.X && p.Y == a.Y }
func (p *Pos) Add(a Pos)               { p.X += a.X; p.Y += a.Y }
func (p *Pos) Subtract(a Pos)          { p.X -= a.X; p.Y -= a.Y }

type FPos struct {
	X float32 `json:"x"`
	Y float32 `json:"y"`
}

func (p FPos) Pos() Pos {
	return Pos{float64(p.X), float64(p.Y)}
}

// iterates through positions, making vector between them, optionally closing
func ForVectors(positions []Pos, close bool, handle func(s Pos, v Pos) bool) {
	var i = 0

	for i < len(positions) {
		s := positions[i]
		e := Pos{}
		if i == len(positions)-1 {
			if close {
				e = positions[0].Minus(s)
			} else {
				break
			}
		} else {
			e = positions[i+1]
		}
		if !handle(s, e.Minus(s)) {
			break
		}
		i++
	}
}

func GetTPositionInPosPath(path []Pos, t float64, close bool) Pos {
	var length = 0.0
	var resultPos = Pos{}

	if t <= 0 {
		return path[0]
	}
	ForVectors(path, close, func(s, v Pos) bool {
		length += v.Length()
		return true
	})
	if t >= 1 {
		if close {
			return path[0]
		}
		return path[len(path)-1]
	}

	tlen := t * length
	length = 0.0

	ForVectors(path, close, func(s, v Pos) bool {
		vlen := v.Length()
		l := length + vlen
		if l >= tlen {
			ldiff := tlen - length
			f := ldiff / vlen
			resultPos = s.Plus(v.TimesD(f))
			return false
		}
		length = l
		return true
	})

	return resultPos
}

func (p Pos) Copy() Pos {
	return p
}

func PosFromAngleDeg(deg float64) Pos {
	return Pos{math.Sin(zmath.DegToRad(deg)), -math.Cos(zmath.DegToRad(deg))}
}

func (p Pos) ToAngleDeg() float64 {
	return zmath.RadToDeg(p.ArcTanToRad())
}

func PosLongLatToMeters(pos1 Pos, pos2 Pos) float64 {
	R := 6371.0 // Radius of the earth in km
	dLat := zmath.DegToRad(pos2.Y - pos1.Y)
	dLon := zmath.DegToRad(pos2.X - pos1.X)
	a := (math.Pow(math.Sin(dLat/2.0), 2.0) + math.Cos(zmath.DegToRad(pos1.Y))) * math.Cos(zmath.DegToRad(pos2.Y)) * math.Pow(math.Sin(dLon/2.0), 2.0)
	c := 2.0 * float64(math.Asin(math.Sqrt(math.Abs(a))))
	return c * R * 1000.0
}

func (pos Pos) ArcTanToRad() float64 {
	var a = float64(math.Atan2(pos.Y, pos.X))
	if a < 0 {
		a += math.Pi * 2
	}
	return a
}

func (p Pos) Floor() Pos {
	return Pos{math.Floor(p.X), math.Floor(p.Y)}
}

func (p Pos) Ceil() Pos {
	return Pos{math.Ceil(p.X), math.Ceil(p.Y)}
}

func (p Pos) Round() Pos {
	return Pos{math.Round(p.X), math.Round(p.Y)}
}

func (p Pos) Average() float64 {
	return (p.X + p.Y) / 2
}

func (p Pos) Value() (driver.Value, error) {
	return []byte(p.String()), nil
}

func (p *Pos) Scan(value interface{}) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	if str == "" {
		*p = PosUndef
		return nil
	}
	pos, err := PosFromString(str)
	if err != nil {
		return err
	}
	*p = pos
	return nil
}

func TSL(s, lang string) string {
	return s
}

func GetHemisphereDirectionsFromGeoAlignment(alignment Alignment, separator, langCode string) string {
	var str = ""
	if alignment&Top != 0 {
		str = TSL("North", langCode) // General name for north as in north-east wind etc
	}
	if alignment&Bottom != 0 {
		str = zstr.Concat(separator, str, TSL("South", langCode)) // General name for south as in south-east wind etc
	}
	if alignment&Left != 0 {
		str = zstr.Concat(separator, str, TSL("West", langCode)) // General name for west as in north-west wind etc
	}
	if alignment&Right != 0 {
		str = zstr.Concat(separator, str, TSL("East", langCode)) // General name for north as in north-east wind etc
	}
	return str
}

type IPos struct {
	X int
	Y int
}

func (p *IPos) ToPos() Pos {
	return PosD(float64(p.X), float64(p.Y))
}

func PosI(x, y int) Pos {
	return PosD(float64(x), float64(y))
}
