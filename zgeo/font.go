package zgeo

import (
	"strings"
)

type FontStyle int

var scale = 1.0

const (
	FontStyleUndef      FontStyle = 0
	FontStyleNormal     FontStyle = 1
	FontStyleBold       FontStyle = 2
	FontStyleItalic     FontStyle = 4
	FontStyleBoldItalic FontStyle = FontStyleBold | FontStyleItalic
)

// TODO: Make font NOT pointer when passing around. Too easy to change something used elsewhere then
type Font struct {
	Name  string    `json:"name"`
	Style FontStyle `json:"style"`
	Size  float64   `json:"size"`
}

type FontSetter interface {
	SetFont(*Font)
}

// DefaultSize This is used
var FontDefaultSize = 14.0
var FontDefaultName = "Helvetica"

func (s FontStyle) String() string {
	var parts []string
	if s&FontStyleNormal != 0 {
		parts = append(parts, "normal")
	}
	if s&FontStyleBold != 0 {
		parts = append(parts, "bold")
	}
	if s&FontStyleItalic != 0 {
		parts = append(parts, "italic")
	}
	if len(parts) == 0 {
		return "normal"
	}
	return strings.Join(parts, " ")
}

func FontStyleFromStr(str string) FontStyle {
	s := FontStyleUndef
	for _, p := range strings.Split(str, " ") {
		switch p {
		case "normal":
			s = FontStyleNormal
		case "bold":
			s |= FontStyleBold
		case "italic":
			s |= FontStyleItalic
		}
	}
	return s
}

func FontNew(name string, size float64, style FontStyle) *Font {
	return &Font{Name: name, Size: size * scale, Style: style}
}

func FontDefault(incSize float64) *Font {
	return FontNew(FontDefaultName, FontDefaultSize+incSize, FontStyleNormal)
}

func FontNice(size float64, style FontStyle) *Font {
	if size <= 0 {
		size = FontDefaultSize + size
	}
	return FontNew(FontDefaultName, size, style)
}

func (f *Font) NewWithSize(size float64) *Font {
	if size < 0 {
		size = f.Size + size
	}
	return FontNew(f.Name, size, f.Style)
}

func (f *Font) NewWithStyle(style FontStyle) *Font {
	return FontNew(f.Name, f.Size, style)
}

func (f *Font) LineHeight() float64 {
	return f.Size
}

func (f *Font) PointSize() float64 {
	return f.Size
}
