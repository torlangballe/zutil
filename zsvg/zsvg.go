package zsvg

import (
	"fmt"
	"html"
	"io"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zdom"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmath"
)

type SVGGenerator struct {
	writer io.Writer
	color  zgeo.Color
	font   *zgeo.Font
}

func NewGenerator(w io.Writer, size zgeo.Size, tag string, font *zgeo.Font) *SVGGenerator {
	s := &SVGGenerator{}
	s.writer = w
	s.color = zgeo.ColorBlack
	f := `<svg xmlns="http://www.w3.org/2000/svg" width="%f" height="%f" role="img" aria-label="%s">` // viewBox="0 0 %fpx %fpx"
	str := fmt.Sprintf(f, size.W, size.H, tag)
	w.Write([]byte(str + "\n"))
	if font != nil {
		// str = fmt.Sprintf(`<style>text { font-family:%s; font-size:%dpx }</style>`, int(zgeo.FontDefaultSize+fontInc))
		kv := zdom.GetFontCSSKeyValues(font)
		// str = fmt.Sprintf(`<style>text { %s }</style>`, zdom.CSSStringFromMap(kv))
		str = fmt.Sprintf(`<style>text { %s }</style>`, zdom.CSSStringFromMap(kv))
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
	f := `<rect width="%f" height="%f" x="%f" y="%f" rx="%f" ry="%f" style="fill:%s"/>` //;stroke-width:3;stroke:red" />
	str := fmt.Sprintf(f, r.Size.W, r.Size.H, r.Pos.X, r.Pos.Y, corner, corner, s.color.Hex())
	s.writer.Write([]byte(str + "\n"))
}

func (s *SVGGenerator) DrawTextAlignedInPos(pos zgeo.Pos, text string, strokeWidth float64, align zgeo.Alignment, angleDeg float64) zmath.RangeF64 {
	size := zcanvas.GetTextSize(text, s.font)
	npos := pos
	htext := html.EscapeString(text)
	anchor := "left"
	if align.Has(zgeo.HorCenter) {
		anchor = "middle"
		npos.X -= size.W / 2
	} else if align.Has(zgeo.Right) {
		anchor = "right"
		npos.X -= size.W
	}
	var geo string
	if angleDeg != 0 {
		geo = fmt.Sprintf(`transform="translate(%d,%d) rotate(%d)"`, int(npos.X), int(npos.Y), int(angleDeg))
	} else {
		geo = fmt.Sprintf(`x="%d" y="%d"`, int(npos.X), int(npos.Y))
	}
	f := `<text %s textAnchor="%s" style="fill:%s">%s</text>` //;stroke-width:3;stroke:red" />
	str := fmt.Sprintf(f, geo, anchor, s.color.Hex(), htext)
	s.writer.Write([]byte(str + "\n"))
	r := zmath.MakeRange(npos.X, npos.X+size.W)
	return r
}

func makeStrokeStyle(col zgeo.Color, width float64) string {
	return fmt.Sprintf(`style="stroke:%s;stroke-width:%f"`, col.Hex(), width)
}

func (s *SVGGenerator) StrokeVertical(x, y1, y2 float64, width float64, ltype zgeo.PathLineType) {
	f := `<line x1="%f" y1="%f" x2="%f" y2="%f" %s/>`
	str := fmt.Sprintf(f, x, y1, x, y2, makeStrokeStyle(s.color, width))
	s.writer.Write([]byte(str + "\n"))
}

func (s *SVGGenerator) StrokeHorizontal(x1, x2, y float64, width float64, ltype zgeo.PathLineType) {
	f := `<line x1="%f" y1="%f" x2="%f" y2="%f" %s/>`
	str := fmt.Sprintf(f, x1, y, x2, y, makeStrokeStyle(s.color, width))
	s.writer.Write([]byte(str + "\n"))
}

func posPairString(pos zgeo.Pos) string {
	return fmt.Sprintf("%d %d", int(pos.X), int(pos.Y))
}

func (s *SVGGenerator) outputPath(path *zgeo.Path, fill, stroke zgeo.Color, width float64) {
	str := "<path"
	// if fill.Valid {
	// str += fmt.Sprintf(` fill="%s"`, fill.Hex())
	str += fmt.Sprintf(` fill="transparent"`)
	// }
	if stroke.Valid {
		str += fmt.Sprintf(` stroke="%s" stroke-width="%d"`, stroke.Hex(), int(width))
	}
	str += ` d="`

	path.ForEachPart(func(part zgeo.PathNode) {
		switch part.Type {
		case zgeo.PathMove:
			str += fmt.Sprintf("M %s ", posPairString(part.Points[0]))
		case zgeo.PathLine:
			str += fmt.Sprintf("L %s ", posPairString(part.Points[0]))
		case zgeo.PathClose:
			str += "Z "
		case zgeo.PathQuadCurve, zgeo.PathCurve:
			break
		}
	})
	str += `"/>`
	s.writer.Write([]byte(str))
}

func (s *SVGGenerator) StrokePath(path *zgeo.Path, width float64, ltype zgeo.PathLineType) {
	s.outputPath(path, zgeo.Color{}, s.color, width)
}
