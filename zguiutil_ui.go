
package zguiutil

func NewBar(title string) (banner *zimageview.ImageView) {
	bar = zcontainer.StackViewHor("bar")
	bar.SetMargin(zgeo.RectFromXY2(6, 0, -6, -3))

	if title != "" {
		label = zlabel.New(title)
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
		label.SetText(title)
	}
	bar.SetDrawHandler(func(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
		y := rect.Max().Y - 3
		r := rect
		r.SetMaxY(y)
		colors := []zgeo.Color{
			zstyle.Col(zgeo.ColorNew(0.85, 0.88, 0.91, 1), zgeo.ColorNew(0.15, 0.18, 0.21, 1)),
			zstyle.Col(zgeo.ColorNew(0.69, 0.72, 0.76, 1), zgeo.ColorNew(0.29, 0.32, 0.36, 1)),
		}
		path := zgeo.PathNewRect(r, zgeo.Size{})
		canvas.DrawGradient(path, colors, r.Min(), r.BottomLeft(), nil)
	})
}
