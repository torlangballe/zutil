//go:build zui

package zhistogram

import (
	"bytes"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zsvg"
)

func NewSVGView(h *Histogram, size zgeo.Size, name string, opts DrawOpts) *zsvg.SVGView {
	buf := bytes.NewBuffer([]byte{})
	var font *zgeo.Font
	if opts.Styling.Font.Size != 0 {
		font = &opts.Styling.Font
	}
	svgGen := zsvg.NewGenerator(buf, size, name, font)
	r := zgeo.Rect{Size: size}
	h.Draw(svgGen, r, opts)
	svg := string(buf.Bytes())
	v := zsvg.NewView(svg, size)
	return v
}
