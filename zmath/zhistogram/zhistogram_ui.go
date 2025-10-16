//go:build zui

package zhistogram

import (
	"bytes"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zsvg"
)

func NewSVGView(h *Histogram, size zgeo.Size, name string, opts DrawOpts) *zsvg.SVGView {
	buf := bytes.NewBuffer([]byte{})
	fontInc := 0.0
	if opts.Styling.Font.Size != 0 {
		fontInc = opts.Styling.Font.Size - zgeo.FontDefaultSize
	}
	svgGen := zsvg.NewGenerator(buf, size, name, fontInc)
	r := zgeo.Rect{Size: size}
	h.Draw(svgGen, r, opts)
	svg := string(buf.Bytes())
	v := zsvg.NewView(svg, size)
	return v
}
