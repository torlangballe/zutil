package zgeo

import (
	"math"
	"strconv"
	"strings"

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
	Valid bool
	Rgba  RGBA
	// Tile
}

var ColorDefaultForeground = ColorBlack
var ColorDefaultBackground = ColorWhite

func ColorNewGray(white, a float32) (c Color) {
	c.Valid = true
	c.Rgba.R = white
	c.Rgba.G = white
	c.Rgba.B = white
	c.Rgba.A = a
	return
}

func ColorNew(r, g, b, a float32) (c Color) {
	c.Valid = true
	c.Rgba.R = r
	c.Rgba.G = g
	c.Rgba.B = b
	c.Rgba.A = a
	return
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
		c.Rgba.R = b
		c.Rgba.G = t
		c.Rgba.B = p

	case 1:
		c.Rgba.R = q
		c.Rgba.G = b
		c.Rgba.B = p

	case 2:
		c.Rgba.R = p
		c.Rgba.G = b
		c.Rgba.B = t

	case 3:
		c.Rgba.R = p
		c.Rgba.G = q
		c.Rgba.B = b

	case 4:
		c.Rgba.R = t
		c.Rgba.G = p
		c.Rgba.B = b

	case 5:
		c.Rgba.R = b
		c.Rgba.G = p
		c.Rgba.B = q
	}
	c.Rgba.A = a
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
	return c.Rgba
}

func (c Color) GetGrayScaleAndAlpha() (float32, float32) { // white, alpha
	return c.GetGrayScale(), c.Rgba.A
}

func (c Color) GetGrayScale() float32 {
	return 0.2126*c.Rgba.R + 0.7152*c.Rgba.G + 0722*c.Rgba.B
}
func (c Color) Opacity() float32 {
	return c.Rgba.A
}

func (c Color) OpacityChanged(opacity float32) Color {
	return ColorNew(c.Rgba.R, c.Rgba.G, c.Rgba.B, opacity)
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
	g := c.GetGrayScale()
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
	if len == 2 {
		n *= 16
	}
	return float32(n) / 255
}

func ColorFromString(str string) Color {
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
	case "lightGray":
		return ColorLightGray
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
			return ColorFromSlice(cols)
		}
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

var ColorWhite = ColorNewGray(1, 1)
var ColorBlack = ColorNewGray(0, 1)
var ColorGray = ColorNewGray(0.5, 1)
var ColorDarkGray = ColorNewGray(0.25, 1)
var ColorLightGray = ColorNewGray(0.75, 1)
var ColorClear = ColorNewGray(0, 0)
var ColorBlue = ColorNew(0, 0, 1, 1)
var ColorRed = ColorNew(1, 0, 0, 1)
var ColorYellow = ColorNew(1, 1, 0, 1)
var ColorGreen = ColorNew(0, 1, 0, 1)
var ColorOrange = ColorNew(1, 0.5, 0, 1)
var ColorCyan = ColorNew(0, 1, 1, 1)
var ColorMagenta = ColorNew(1, 0, 1, 1)
