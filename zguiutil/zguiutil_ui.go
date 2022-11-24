//go:build zui

package zguiutil

import (
	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

func NewBar(title string) *zcontainer.StackView {
	bar := zcontainer.StackViewHor("bar")
	bar.SetMargin(zgeo.RectFromXY2(6, 0, -6, -3))

	if title != "" {
		label := zlabel.New(title)
		label.SetObjectName("title")
		label.SetFont(zgeo.FontNew("Arial", 18, zgeo.FontStyleNormal))
		label.SetColor(zstyle.DefaultFGColor())
		label.SetMaxWidth(500)
		label.SetMaxLines(1)
		label.SetLongPressedHandler(func() {
			zlog.PrintAllGoroutines()
			ztimer.DumpRepeaters()
		})
		bar.Add(label, zgeo.Left|zgeo.VertCenter|zgeo.HorExpand, zgeo.Size{0, 0})
	}
	bar.SetDrawHandler(func(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
		colors := []zgeo.Color{
			zstyle.Col(zgeo.ColorNew(0.85, 0.88, 0.91, 1), zgeo.ColorNew(0.15, 0.18, 0.21, 1)),
			zstyle.Col(zgeo.ColorNew(0.69, 0.72, 0.76, 1), zgeo.ColorNew(0.29, 0.32, 0.36, 1)),
		}
		path := zgeo.PathNewRect(rect, zgeo.Size{})
		canvas.DrawGradient(path, colors, rect.Min(), rect.BottomLeft(), nil)
	})
	return bar
}
