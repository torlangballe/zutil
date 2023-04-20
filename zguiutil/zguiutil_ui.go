//go:build zui

package zguiutil

import (
	"time"

	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlocale"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
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

func Labelize(view zview.View, prefix string, minWidth float64, alignment zgeo.Alignment) (label *zlabel.Label, stack *zcontainer.StackView, viewCell *zcontainer.Cell) {
	font := zgeo.FontNice(zgeo.FontDefaultSize, zgeo.FontStyleBold)
	to, _ := view.(ztextinfo.Owner)
	if to != nil {
		ti := to.GetTextInfo()
		font = ti.Font
		font.Style = zgeo.FontStyleBold
	}
	title := prefix
	checkBox, isCheck := view.(*zcheckbox.CheckBox)
	if checkBox != nil && alignment&zgeo.Right != 0 {
		title = ""
		_, cstack := zcheckbox.Labelize(checkBox, prefix)
		view = cstack
		alignment = alignment.FlippedHorizontal()
	}
	label = zlabel.New(title)
	label.SetObjectName("$labelize.label " + prefix)
	label.SetTextAlignment(zgeo.Right)
	label.SetFont(font)
	label.SetColor(zstyle.DefaultFGColor().WithOpacity(0.7))
	stack = zcontainer.StackViewHor("$labelize." + prefix) // give it special name so not easy to mis-search for in recursive search

	stack.Add(label, zgeo.CenterLeft).MinSize.W = minWidth
	marg := zgeo.Size{}
	if isCheck {
		marg.W = -6 // in html cell has a box around it of 20 pixels
	}
	// zlog.Info("Labelize view:", view.ObjectName(), alignment, marg)
	viewCell = stack.Add(view, alignment, marg)
	return
}

func NewCurrentTimeLabel() *zlabel.Label {
	label := zlabel.New("")
	label.SetObjectName("time")
	label.SetFont(zgeo.FontDefault().NewWithSize(zgeo.FontDefaultSize - 2))
	label.SetColor(zgeo.ColorNewGray(0.5, 1))
	label.SetMinWidth(145)
	label.SetTextAlignment(zgeo.Right)
	label.SetPressedDownHandler(func() {
		toggleTimeZoneMode(label)
	})
	updateCurrentTime(label)
	ztimer.RepeatForever(1, func() {
		updateCurrentTime(label)
	})
	return label
}

func toggleTimeZoneMode(label *zlabel.Label) {
	d := !zlocale.DisplayServerTime.Get()
	zlog.Info("toggleTimeZoneMode", d)
	zlocale.DisplayServerTime.Set(d)
	updateCurrentTime(label)
}

func updateCurrentTime(label *zlabel.Label) {
	t := time.Now()
	t = t.Add(zapp.ServerTimeDifference)
	if zapp.ServerTimezoneName != "" {
		loc, _ := time.LoadLocation(zapp.ServerTimezoneName)
		if loc != nil {
			t = t.In(loc)
		}
	}
	str := ztime.GetNice(time.Now(), true)
	label.SetText(str)
	col := zgeo.ColorBlack
	if zapp.ServerTimeDifference > time.Second*4 {
		col = zgeo.ColorRed
	}
	label.SetColor(col.WithOpacity(0.7))
}
