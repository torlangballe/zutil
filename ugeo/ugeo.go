package ugeo

import (
	"bytes"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"

	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zstr/ulocale"
	"github.com/torlangballe/zutil/zmath"
)

const KDegToMeters = 1113200.0

// FPoint *****************************************************

type FPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type FPolygon []FPoint

type FSize struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type FRect struct {
	Min FPoint `json:"min"`
	Max FPoint `json:"max"`
}

// FSize *****************************************************

func (s FSize) IsNull() bool {
	return s.W == 0 && s.H == 0
}

func (s FSize) ScaledInto(in FSize) FSize {
	if s.W == 0 || s.H == 0 || in.W == 0 || in.H == 0 {
		return FSize{}
	}
	min := math.Min(in.W/s.W, in.H/s.H)
	scaled := FSize{s.W * min, s.H * min}

	return scaled
}

// FPoint *****************************************************

func (f FPoint) IsNull() bool {
	return f.X == 0 && f.Y == 0
}

func (r FPoint) Add(a FPoint) FPoint {
	return FPoint{r.X + a.X, r.Y + a.Y}
}

func (r FPoint) Sub(a FPoint) FPoint {
	return FPoint{r.X - a.X, r.Y - a.Y}
}

func (r FPoint) Sign() FPoint {
	return FPoint{zmath.Sign(r.X), zmath.Sign(r.Y)}
}

func (p FPoint) Length() float64 {
	return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

func (p *FPoint) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "POINT(%f,%f)", p.Y, p.X)
	return buf.Bytes(), nil
}

func (p *FPoint) String() string {
	return fmt.Sprintf("POINT(%f,%f)", p.Y, p.X)
}

// FRect *****************************************************

func (r FRect) Size() FSize {
	return FSize{r.Max.X - r.Min.X, r.Max.Y - r.Min.Y}
}

func (r FRect) Width() float64 {
	return r.Max.X - r.Min.X
}

func (r FRect) Height() float64 {
	return r.Max.Y - r.Min.Y
}

func (r FRect) Center() FPoint {
	return FPoint{(r.Min.X + r.Max.X) / 2, (r.Min.Y + r.Max.Y) / 2}
}

func (r *FRect) Value() (driver.Value, error) {
	if r == nil {
		return nil, nil
	}
	return json.Marshal(r)
}

func (r *FRect) Scan(val interface{}) error {
	data, ok := val.([]byte)
	if !ok {
		return errors.New("FRect sql Scan unsupported data type")
	}
	return json.Unmarshal(data, r)
}

func AbsFRect(minx, miny, maxx, maxy float64) (r FRect) {
	r.Min.X = minx
	r.Min.Y = miny
	r.Max.X = maxx
	r.Max.Y = maxy

	return
}

func NewFRect(minx, miny, maxx, maxy float64) FRect {
	return FRect{Min: FPoint{minx, miny}, Max: FPoint{maxx, maxy}}
}

func NewFRectFromPointSize(pos FPoint, size FSize) FRect {
	return FRect{Min: pos, Max: FPoint{pos.X + size.W, pos.Y + size.H}}
}

func (r FRect) Contains(p FPoint) bool {
	return p.X >= r.Min.X && p.Y >= r.Min.Y && p.X <= r.Max.X && p.Y <= r.Max.Y
}

func (r FRect) OverlapWith(a FRect) (out FRect) {
	out.Min.X = math.Max(r.Min.X, a.Min.X)
	out.Max.X = math.Max(math.Min(r.Max.X, a.Max.X), out.Min.X)
	out.Min.Y = math.Max(r.Min.Y, a.Min.Y)
	out.Max.Y = math.Max(math.Min(r.Max.Y, a.Max.Y), out.Min.Y)
	return
}

// Misc ****************************************

func hsin(theta float64) float64 {
	return math.Pow(math.Sin(theta/2), 2)
}

func GetMeterVectorInDegreesFromPos(posDeg FPoint, vectorMeters FPoint) (vectorDeg FPoint) {
	xFactor := KDegToMeters * math.Cos(posDeg.Y*(math.Pi/180))
	add := FPoint{vectorMeters.X * xFactor, vectorMeters.Y}
	return posDeg.Add(add)
}

func (p FPoint) DistanceInMetersFromDegrees(to FPoint) float64 {
	la1 := p.Y * math.Pi / 180
	lo1 := p.X * math.Pi / 180
	la2 := to.Y * math.Pi / 180
	lo2 := to.X * math.Pi / 180
	r := 6378100.0 // Earth radius in METERS
	h := hsin(la2-la1) + math.Cos(la1)*math.Cos(la2)*hsin(lo2-lo1)
	return 2 * r * math.Asin(math.Sqrt(h))
}

func GetDistanceAsString(meters float64, metric bool, langCode string, round bool) string {
	type DistanceEnum uint

	const (
		METER DistanceEnum = iota
		KM
		MILE
		YARD
	)
	dtype := METER
	d := meters
	distance := ""
	word := ""

	if metric {
		if d >= 1000 {
			dtype = KM
			d /= 1000
		}
	} else {
		d /= 1.0936133
		if d >= 1760 {
			dtype = MILE
			d /= 1760
			distance = fmt.Sprintf("%.1lf", d)
		} else {
			dtype = YARD
			d = math.Floor(d)
			distance = strconv.FormatInt(int64(d), 10)
		}
	}
	switch dtype {
	case METER:
		word = ulocale.GetMeter(true, langCode)

	case KM:
		word = ulocale.GetKiloMeter(true, langCode)

	case MILE:
		word = ulocale.GetMile(true, langCode)

	case YARD:
		word = ulocale.GetYard(true, langCode)
	}
	if dtype == METER || dtype == YARD && round {
		d = math.Ceil(((math.Ceil(d) + 9) / 10) * 10)
		distance = strconv.FormatInt(int64(d), 10)
	} else if round && d > 50 {
		distance = fmt.Sprintf("%d", int(d))
	} else {
		distance = fmt.Sprintf("%.1lf", d)
	}
	return distance + " " + word
}

func GetHemisphereDirectionsFromGeoDirection(dir FPoint, separator string, langCode string) string {
	str := ""
	if dir.Y == 1 {
		str = ulocale.GetNorth(langCode)
	}
	if dir.Y == -1 {
		zstr.Concat(separator, str, ulocale.GetSouth(langCode))
	}
	if dir.X == -1 {
		zstr.Concat(separator, str, ulocale.GetWest(langCode))
	}
	if dir.X == 1 {
		zstr.Concat(separator, str, ulocale.GetEast(langCode))
	}
	return str
}
