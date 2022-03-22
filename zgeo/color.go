package zgeo

import (
	"fmt"
	"image/color"
	"math"
	"math/rand"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

//  Created by Tor Langballe on 07-June-2019
//

// Check out: https://github.com/muesli/gamut

type HSBA struct {
	H float32 `json:"H"`
	S float32 `json:"S"`
	B float32 `json:"B"`
	A float32 `json:"A"`
}

type RGBA struct {
	R float32 `json:"R"`
	G float32 `json:"G"`
	B float32 `json:"B"`
	A float32 `json:"A"`
}

type Color struct {
	Valid  bool
	Colors RGBA
}

func ColorF(c Color) func() Color {
	return func() Color { return c }
}

// RGBA returns r, g, b, a as 0-255 uint32 values.
// Makes it conform to image/color Color interface
func (c Color) RGBA() (r, g, b, a uint32) {
	af := c.Colors.A * 0xFFFF
	r = uint32(c.Colors.R * af)
	g = uint32(c.Colors.G * af)
	b = uint32(c.Colors.B * af)
	return r, g, b, uint32(af)

	//	return uint32(c.Colors.R * 0xffff), uint32(c.Colors.G * 255), uint32(c.Colors.B * 255), uint32(c.Colors.A * 255)
}

func ColorNewGray(white, a float32) (c Color) {
	c.Valid = true
	w := zfloat.Clamped32(white, 0, 1)
	c.Colors.R = w
	c.Colors.G = w
	c.Colors.B = w
	c.Colors.A = zfloat.Clamped32(a, 0, 1)
	return
}

func ColorNew(r, g, b, a float32) (c Color) {
	c.Valid = true
	c.Colors.R = zfloat.Clamped32(r, 0, 1)
	c.Colors.G = zfloat.Clamped32(g, 0, 1)
	c.Colors.B = zfloat.Clamped32(b, 0, 1)
	c.Colors.A = zfloat.Clamped32(a, 0, 1)
	return
}

func ColorNew255(r, g, b, a int) (c Color) {
	return ColorNew(float32(r)/255, float32(g)/255, float32(b)/255, float32(a)/255)
}

func (c Color) Slice32() []float32 {
	slice := make([]float32, 4, 4)
	slice[0] = c.Colors.R
	slice[1] = c.Colors.G
	slice[2] = c.Colors.B
	slice[3] = c.Colors.A
	return slice
}

func (c Color) SliceInt255() []int {
	slice := make([]int, 4, 4)
	slice[0] = int(c.Colors.R * 255)
	slice[1] = int(c.Colors.G * 255)
	slice[2] = int(c.Colors.B * 255)
	slice[3] = int(c.Colors.A * 255)
	return slice
}

// ColorFromSlice returns a opaque gray for 1 item, graya for 2, opaque rgb for 3, and rgba for 4
func ColorFromSlice(s []float32) Color {
	switch len(s) {
	case 1:
		return ColorNewGray(s[0], 1)
	case 2:
		return ColorNewGray(s[0], s[1])
	case 3:
		return ColorNew(s[0], s[1], s[2], 1)
	case 4:
		return ColorNew(s[0], s[1], s[2], s[3])
	}
	return Color{}
}

func ColorNewHSBA(hsba HSBA) (c Color) {
	// hsba.S = 1 - hsba.S
	c.Valid = true
	i := (hsba.H * 360.0) / 60.0
	f := i - float32(int(i))

	p := hsba.B * (1 - hsba.S)
	q := hsba.B * (1 - f*hsba.S)
	t := hsba.B * (1 - (1-f)*hsba.S)

	switch int(hsba.H*6) % 6 {
	case 0:
		c.Colors.R, c.Colors.G, c.Colors.B = hsba.B, t, p
	case 1:
		c.Colors.R, c.Colors.G, c.Colors.B = q, hsba.B, p
	case 2:
		c.Colors.R, c.Colors.G, c.Colors.B = p, hsba.B, t
	case 3:
		c.Colors.R, c.Colors.G, c.Colors.B = p, q, hsba.B
	case 4:
		c.Colors.R, c.Colors.G, c.Colors.B = t, p, hsba.B
	case 5:
		c.Colors.R, c.Colors.G, c.Colors.B = hsba.B, p, q
	}
	c.Colors.A = hsba.A
	return
}

func (c Color) HSBA() HSBA {
	var h, s, b, a float32
	var hsba HSBA
	hsba.H = float32(h)
	hsba.S = float32(s)
	hsba.B = float32(b)
	hsba.A = float32(a)
	return hsba
}

func (c Color) RGBAValue() RGBA {
	return c.Colors
}

func (c Color) GrayScaleAndAlpha() (float32, float32) { // white, alpha
	return c.GrayScale(), c.Colors.A
}

func (c Color) GrayScale() float32 {
	return 0.2126*c.Colors.R + 0.7152*c.Colors.G + 0.722*c.Colors.B
}
func (c Color) Opacity() float32 {
	return c.Colors.A
}

func (c *Color) SetOpacity(opacity float32) {
	c.Colors.A = opacity
}

func (c Color) WithOpacity(opacity float32) Color {
	return ColorNew(c.Colors.R, c.Colors.G, c.Colors.B, opacity)
}

func (c Color) Mixed(withColor Color, amount float32) Color {
	wc := withColor.RGBAValue()
	col := c.RGBAValue()
	amount *= wc.A
	r := (1-amount)*col.R + wc.R*amount
	g := (1-amount)*col.G + wc.G*amount
	b := (1-amount)*col.B + wc.B*amount
	a := c.Colors.A * withColor.Colors.A
	return ColorNew(r, g, b, a)
}

func (c Color) MultipliedBrightness(multiply float32) Color {
	hsba := c.HSBA()
	return ColorNewHSBA(hsba)
}

func (c Color) AlteredContrast(contrast float32) Color {
	multi := float32(math.Pow((float64(1+contrast))/1, 2.0))
	var col = c.RGBAValue()
	col.R = (col.R-0.5)*multi + 0.5
	col.G = (col.G-0.5)*multi + 0.5
	col.B = (col.B-0.5)*multi + 0.5
	return ColorNew(col.R, col.G, col.B, col.A)
}

func (c Color) ContrastingGray() Color {
	g := c.GrayScale()
	if g < 0.5 {
		return ColorWhite
	}
	return ColorBlack
}

// Difference returns the difference brween c and a, where 0 is same, 1 is black to white. Opacity is used, so each 4 components contribute 25% each
func (c Color) Difference(a Color) float32 {
	if c.Valid != a.Valid {
		return 1
	}
	dr := math.Abs(float64(c.Colors.R - a.Colors.R))
	dg := math.Abs(float64(c.Colors.G - a.Colors.G))
	db := math.Abs(float64(c.Colors.B - a.Colors.B))
	da := math.Abs(float64(c.Colors.A - a.Colors.A))
	return float32(dr+dg+db+da) / 4
}

func getHexAsValue(str string, len int) float32 {
	n, err := strconv.ParseInt(str, 16, 32)
	if err != nil {
		zlog.Error(err, "parse")
		return -1
	}
	if len == 1 {
		n *= 16
	}
	return float32(n) / 255
}

func (c Color) Hex() string {
	return fmt.Sprintf("#%02x%02x%02x%02x", int(c.Colors.R*255), int(c.Colors.G*255), int(c.Colors.B*255), int(c.Colors.A*255))
}

func ColorFromString(str string) Color {
	// zlog.Info("ColorFromString:", str)
	switch str {
	case "initial":
		return Color{}
	case "red":
		return ColorRed
	case "white":
		return ColorWhite
	case "black":
		return ColorBlack
	case "gray":
		return ColorGray
	case "darkGray":
		return ColorDarkGray
	case "clear":
		return ColorClear
	case "blue":
		return ColorBlue
	case "yellow":
		return ColorYellow
	case "green":
		return ColorGreen
	case "orange":
		return ColorOrange
	case "cyan":
		return ColorCyan
	case "magenta":
		return ColorMagenta
	case "silver":
		return ColorSilver
	case "maroon":
		return ColorMaroon
	case "olive":
		return ColorOlive
	case "lime":
		return ColorLime
	case "aqua":
		return ColorAqua
	case "teal":
		return ColorTeal
	case "navy":
		return ColorNavy
	case "fuchsia":
		return ColorFuchsia
	case "purple":
		return ColorPurple
	case "pink":
		return ColorPink
	}
	if zstr.HasPrefix(str, "rgba(", &str) || zstr.HasPrefix(str, "rgb(", &str) {
		if !zstr.HasSuffix(str, ")", &str) {
			return Color{}
		}
		var r, g, b, a float32
		a = 1
		for i, p := range strings.Split(str, ",") {
			p = strings.TrimSpace(p)
			f, _ := strconv.ParseFloat(p, 32)
			f32 := float32(f)
			switch i {
			case 0:
				r = f32 / 255
			case 1:
				g = f32 / 255
			case 2:
				b = f32 / 255
			case 3:
				a = f32
			default:
				zlog.Info("Too many parts")
				return Color{}
			}
		}
		return ColorNew(r, g, b, a)
	} else if zstr.HasPrefix(str, "#", &str) {
		slen := len(str)
		switch slen {
		case 1, 2:
			g := getHexAsValue(str, slen)
			return ColorNewGray(g, 1)
		case 3, 4:
			var a float32 = 1
			r := getHexAsValue(str[0:1], 1)
			g := getHexAsValue(str[1:2], 1)
			b := getHexAsValue(str[2:3], 1)
			if slen == 4 {
				a = getHexAsValue(str[3:4], 1)
			}
			return ColorNew(r, g, b, a)
		case 6, 8:
			var a float32 = 1
			r := getHexAsValue(str[0:2], 2)
			g := getHexAsValue(str[2:4], 2)
			b := getHexAsValue(str[4:6], 2)
			if slen == 8 {
				a = getHexAsValue(str[6:8], 2)
			}
			return ColorNew(r, g, b, a)
		}
	}
	zlog.Error(nil, "bad color string", str, zlog.GetCallingStackString())
	return Color{}
}

func ColorFromGo(c color.Color) Color {
	r, g, b, a := c.RGBA()
	af := float32(a) / 0xFFFF
	if af == 0 {
		return ColorClear
	}
	return ColorNew(float32(r)/0xFFFF/af, float32(g)/0xFFFF/af, float32(b)/0xFFFF/af, af)
}

func (c Color) GoColor() (gcol color.NRGBA64) {
	gcol.R = uint16(c.Colors.R * 0xFFFF)
	gcol.G = uint16(c.Colors.G * 0xFFFF)
	gcol.B = uint16(c.Colors.B * 0xFFFF)
	gcol.A = uint16(c.Colors.A * 0xFFFF)
	return
}

func ColorRandom() Color {
	return ColorNew(rand.Float32(), rand.Float32(), rand.Float32(), 1)
}

var ColorDarkGray = ColorNewGray(0.25, 1)
var ColorLightGray = ColorNewGray(0.75, 1)
var ColorClear = ColorNewGray(0, 0)
var ColorOrange = ColorNew(1, 0.5, 0, 1)
var ColorCyan = ColorNew(0, 1, 1, 1)
var ColorMagenta = ColorNew(1, 0, 1, 1)

var ColorWhite = ColorNewGray(1, 1)
var ColorSilver = ColorNew(0.75, 0.75, 0.75, 1)
var ColorGray = ColorNewGray(0.5, 1)
var ColorBlack = ColorNewGray(0, 1)
var ColorRed = ColorNew(1, 0, 0, 1)
var ColorMaroon = ColorNew(0.5, 0, 0, 1)
var ColorYellow = ColorNew(1, 1, 0, 1)
var ColorOlive = ColorNew(0.5, 0.5, 0, 1)
var ColorLime = ColorNew(0, 1, 0, 1)
var ColorGreen = ColorNew(0, 1, 0, 1)
var ColorDarkGreen = ColorNew(0, 0.5, 0, 1)
var ColorAqua = ColorNew(0, 1, 1, 1)
var ColorTeal = ColorNew(0, 0.5, 0.5, 1)
var ColorBrown = ColorNew(0.65, 0.26, 0.16, 1)
var ColorBlue = ColorNew(0, 0, 1, 1)
var ColorNavy = ColorNew(0, 0, 0.5, 1)
var ColorFuchsia = ColorNew(1, 0, 1, 1)
var ColorPurple = ColorNew(0.5, 0, 0.5, 1)
var ColorPink = ColorNew(1, 0.8, 0.8, 1)

var ColorDistinctList = []Color{ColorOrange, ColorCyan, ColorMagenta, ColorGray, ColorBlack, ColorRed, ColorMaroon, ColorYellow, ColorOlive, ColorLime, ColorGreen, ColorDarkGreen, ColorAqua, ColorTeal, ColorBlue, ColorNavy, ColorDarkGray, ColorFuchsia, ColorPurple, ColorLightGray}

type DropShadow struct {
	Delta Size
	Blur  float32
	Color Color
}

var DropShadowDefault = DropShadow{Delta: Size{3, 3}, Blur: 3, Color: ColorBlack}

func KelvinToColor(kelvin float32) Color {
	var r, g, b int
	if kelvin == 6500 {
		return ColorWhite
	}
	temperature := float64(kelvin) * 0.01
	if kelvin < 6500 {
		r = 255
		// a + b x + c Log[x] /.
		// {a -> -155.25485562709179`,
		// b -> -0.44596950469579133`,
		// c -> 104.49216199393888`,
		// x -> (kelvin/100) - 2}
		green := temperature - 2
		g = floatToUint8(-155.25485562709179 - 0.44596950469579133*green + 104.49216199393888*math.Log(green))
		if kelvin > 2000 {
			// a + b x + c Log[x] /.
			// {a -> -254.76935184120902`,
			// b -> 0.8274096064007395`,
			// c -> 115.67994401066147`,
			// x -> kelvin/100 - 10}
			blue := temperature - 10
			b = floatToUint8(-254.76935184120902 + 0.8274096064007395*blue + 115.67994401066147*math.Log(blue))
		}
	} else {
		b = 255
		// a + b x + c Log[x] /.
		// {a -> 351.97690566805693`,
		// b -> 0.114206453784165`,
		// c -> -40.25366309332127
		//x -> (kelvin/100) - 55}
		red := temperature - 55.
		r = floatToUint8(351.97690566805693 + 0.114206453784165*red - 40.25366309332127*math.Log(red))
		// a + b x + c Log[x] /.
		// {a -> 325.4494125711974`,
		// b -> 0.07943456536662342`,
		// c -> -28.0852963507957`,
		// x -> (kelvin/100) - 50}
		green := temperature - 50.
		g = floatToUint8(325.4494125711974 + 0.07943456536662342*green - 28.0852963507957*math.Log(green))
	}
	return ColorNew255(r, g, b, 255)
}

func floatToUint8(x float64) int {
	if x >= 254.4 {
		return 255
	}
	if x <= 0. {
		return 0
	}
	return int(math.Ceil(x + 0.5))
}
