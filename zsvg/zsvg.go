package zsvg

import (
	"fmt"
	"html"
	"io"

	"github.com/torlangballe/zui/zdom"
	"github.com/torlangballe/zutil/zgeo"
)

type SVGGenerator struct {
	writer io.Writer
	color  zgeo.Color
	font   *zgeo.Font
}

func NewGenerator(w io.Writer, size zgeo.Size, tag string, fontInc float64) *SVGGenerator {
	s := &SVGGenerator{}
	s.writer = w
	s.color = zgeo.ColorBlack
	f := `<svg xmlns="http://www.w3.org/2000/svg" width="%f" height="%f" role="img" aria-label="%s">` // viewBox="0 0 %fpx %fpx"
	str := fmt.Sprintf(f, size.W, size.H, tag)
	w.Write([]byte(str + "\n"))
	if fontInc != 0 {
		str = fmt.Sprintf(`<style>text { font-family:Arial; font-size:%dpx }</style>`, int(zgeo.FontDefaultSize+fontInc))
		w.Write([]byte(str + "\n"))
	}
	return s
}

// func NewByteSVGGenerator(size zgeo.Size, tag string) *SVGGenerator {
// 	buf := bytes.NewBuffer([]byte{})
// 	return NewSVGGenerator(buf, size, tag)
// }

// func (s *SVGGenerator) BufferedString() string {
// 	b := s.writer.(*bytes.Buffer)
// 	return string(b.Bytes())
// }

func (s *SVGGenerator) End() {
	str := `</svg>`
	s.writer.Write([]byte(str + "\n"))
}

func (s *SVGGenerator) SetColor(col zgeo.Color) {
	s.color = col
}

func (s *SVGGenerator) SetFont(font *zgeo.Font, matrix *zgeo.Matrix) error {
	s.font = font
	return nil
}

func (s *SVGGenerator) PushState() {
}

func (s *SVGGenerator) PopState() {
}

func (s *SVGGenerator) ClipPath(path *zgeo.Path, eofill bool) {

}

func (s *SVGGenerator) FillRect(r zgeo.Rect, corner float64) {
	var sfont string
	if s.font != nil {
		sfont = zdom.GetFontStyle(s.font)
	}
	f := `<rect width="%f" height="%f" x="%f" y="%f" rx="%f" ry="%f" style="fill:%s; %s"/>` //;stroke-width:3;stroke:red" />
	str := fmt.Sprintf(f, r.Size.W, r.Size.H, r.Pos.X, r.Pos.Y, corner, corner, s.color.Hex(), sfont)
	s.writer.Write([]byte(str + "\n"))
}

func (s *SVGGenerator) DrawTextAlignedInPos(pos zgeo.Pos, text string, strokeWidth float64, align zgeo.Alignment) {
	htext := html.EscapeString(text)
	anchor := "middle"
	if align.Has(zgeo.Left) {
		anchor = "left"
	} else if align.Has(zgeo.Right) {
		anchor = "right"
	}
	f := `<text x="%f" y="%f" text-anchor="%s" style="fill:%s">%s</text>` //;stroke-width:3;stroke:red" />
	str := fmt.Sprintf(f, pos.X, pos.Y, anchor, s.color.Hex(), htext)
	s.writer.Write([]byte(str + "\n"))
}
