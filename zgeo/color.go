package zgeo

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

//  Created by Tor Langballe on 07-June-2019
//

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

var ColorDefaultForeground = ColorBlack
var ColorDefaultBackground = ColorWhite

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

func ColorNewHSBA(h, s, b, a float32) (c Color) {
	c.Valid = true

	i, _ := math.Modf(float64(h * 6))
	var f = h*6 - float32(i)
	var p = b * (1 - s)
	var q = b * (1 - f*s)
	var t = b * (1 - (1-f)*s)

	switch math.Mod(i, 6) {
	case 0:
		c.Colors.R = b
		c.Colors.G = t
		c.Colors.B = p

	case 1:
		c.Colors.R = q
		c.Colors.G = b
		c.Colors.B = p

	case 2:
		c.Colors.R = p
		c.Colors.G = b
		c.Colors.B = t

	case 3:
		c.Colors.R = p
		c.Colors.G = q
		c.Colors.B = b

	case 4:
		c.Colors.R = t
		c.Colors.G = p
		c.Colors.B = b

	case 5:
		c.Colors.R = b
		c.Colors.G = p
		c.Colors.B = q
	}
	c.Colors.A = a
	return
}

func (c Color) GetHSBA() HSBA {
	var h, s, b, a float32
	var hsba HSBA
	hsba.H = float32(h)
	hsba.S = float32(s)
	hsba.B = float32(b)
	hsba.A = float32(a)
	return hsba
}

func (c Color) GetRGBA() RGBA {
	return c.Colors
}

func (c Color) GrayScaleAndAlpha() (float32, float32) { // white, alpha
	return c.GrayScale(), c.Colors.A
}

func (c Color) GrayScale() float32 {
	return 0.2126*c.Colors.R + 0.7152*c.Colors.G + 0722*c.Colors.B
}
func (c Color) Opacity() float32 {
	return c.Colors.A
}

func (c Color) OpacityChanged(opacity float32) Color {
	return ColorNew(c.Colors.R, c.Colors.G, c.Colors.B, opacity)
}

func (c Color) Mix(withColor Color, amount float32) Color {
	wc := withColor.GetRGBA()
	col := c.GetRGBA()
	r := (1-amount)*col.R + wc.R*amount
	g := (1-amount)*col.G + wc.G*amount
	b := (1-amount)*col.B + wc.B*amount
	a := (1-amount)*col.A + wc.A*amount
	return ColorNew(r, g, b, a)
}

func (c Color) MultipliedBrightness(multiply float32) Color {
	hsba := c.GetHSBA()
	return ColorNewHSBA(hsba.H, hsba.S, hsba.B*multiply, hsba.A)
}

func (c Color) AlteredContrast(contrast float32) Color {
	multi := float32(math.Pow((float64(1+contrast))/1, 2.0))
	var col = c.GetRGBA()
	col.R = (col.R-0.5)*multi + 0.5
	col.G = (col.G-0.5)*multi + 0.5
	col.B = (col.B-0.5)*multi + 0.5
	return ColorNew(col.R, col.G, col.B, col.A)
}

func (c Color) GetContrastingGray() Color {
	g := c.GrayScale()
	if g < 0.5 {
		return ColorWhite
	}
	return ColorBlack
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

func (c Color) GetHex() string {
	return fmt.Sprintf("#%02x%02x%02x%02x", int(c.Colors.R*255), int(c.Colors.G*255), int(c.Colors.B*255), int(c.Colors.A*255))
}

func ColorFromString(str string) Color {
	// zlog.Info("ColorFromString:", str)
	switch str {
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
	}
	if zstr.HasPrefix(str, "rgba(", &str) || zstr.HasPrefix(str, "rgb(", &str) {
		if !zstr.HasSuffix(str, ")", &str) {
			return Color{}
		}
		parts := strings.Split(str, ",")
		if len(parts) != 4 && len(parts) != 3 {
			return Color{}
		}
		var cols = make([]float32, len(parts), len(parts))
		for i, p := range parts {
			p = strings.TrimSpace(p)
			f, err := strconv.ParseFloat(p, 32)
			if err != nil {
				zlog.Error(err)
				return Color{}
			}
			cols[i] = float32(f) / 255
		}
		return ColorFromSlice(cols)
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
	zlog.Error(nil, "bad color string", str)
	return Color{}
}

var ColorDarkGray = ColorNewGray(0.25, 1)
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
var ColorGreen = ColorNew(0, 0.5, 0, 1)
var ColorAqua = ColorNew(0, 1, 1, 1)
var ColorTeal = ColorNew(0, 0.5, 0.5, 1)
var ColorBlue = ColorNew(0, 0, 1, 1)
var ColorNavy = ColorNew(0, 0, 0.5, 1)
var ColorFuchsia = ColorNew(1, 0, 1, 1)
var ColorPurple = ColorNew(0.5, 0, 0.5, 1)
