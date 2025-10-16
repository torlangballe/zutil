//go:build zui

package zsvg

import (
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zgeo"
)

type SVGView struct {
	zview.NativeView
	size zgeo.Size
}

func NewView(svg string, size zgeo.Size) *SVGView {
	v := &SVGView{}
	v.MakeElementFromHTML(v, svg)
	v.SetObjectName("zsvg")
	v.size = size
	return v
}

func (v *SVGView) CalculatedSize(total zgeo.Size) (s, max zgeo.Size) {
	return v.size, v.size
}

// svg := zcanvas.NewSVGGenerator(file, zgeo.SizeD(400, 300), "histo")
// 	h.Draw(svg, zgeo.Rect{Size: s}, opts)
// 	svg.End()
