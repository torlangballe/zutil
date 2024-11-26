package zdraw

import (
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/zwords"
)

func DrawAmountPie(rect zgeo.Rect, canvas *zcanvas.Canvas, value, strokeWidth float64, color, strokeColor zgeo.Color) {
	path := zgeo.PathNew()
	s := rect.Size.MinusD(strokeWidth).DividedByD(2).MinusD(1)
	w := s.Min()
	path.MoveTo(rect.Center())
	path.ArcDegFromCenter(rect.Center(), zgeo.SizeBoth(w), 0, value*360)
	canvas.SetColor(color)
	canvas.FillPath(path)
	line := zgeo.PathNew()
	line.ArcDegFromCenter(rect.Center(), zgeo.SizeBoth(w), 0, 360)
	canvas.SetColor(strokeColor)
	canvas.StrokePath(line, strokeWidth, zgeo.PathLineRound)
}

func StrokeVertInImage(img zimage.SetableImage, x, y1, y2 int, col color.Color) {
	clear := zgeo.ColorClear.GoColor()
	for y := 0; y <= y1; y++ {
		img.Set(x, y, clear)
	}
	for y := y1; y <= y2; y++ {
		img.Set(x, y, col)
	}
}

func MergeImages(box zgeo.Size, images []*zimage.ImageGetter, done func(img *zimage.Image)) {
	zimage.GetImages(images, false, func(all bool) {
		if !all {
			zlog.Error("Not all images got")
			return
		}
		if box.IsNull() {
			for _, ig := range images {
				box.Maximize(ig.Image.Size())
			}
		}
		canvas := zcanvas.New()
		canvas.SetSize(box)
		for _, ig := range images {
			r := zgeo.Rect{Size: box}.Align(ig.Image.Size(), ig.Alignment, ig.Margin)
			canvas.DrawImageAt(ig.Image, r.Pos, false, ig.Opacity)
		}
		canvas.ZImage(false, done)
	})
}

func XToTime(rect zgeo.Rect, x float64, start, end time.Time) time.Time {
	tdiff := ztime.DurSeconds(end.Sub(start))
	dur := ztime.SecondsDur((x - rect.Pos.X) / rect.Size.W * tdiff)
	return start.Add(dur)
}

func TimeToX(rect zgeo.Rect, t, start, end time.Time) float64 {
	diff := ztime.DurSeconds(end.Sub(start))
	return rect.Min().X + ztime.DurSeconds(t.Sub(start))*rect.Size.W/diff
}

func DrawHorTimeAxis(canvas *zcanvas.Canvas, rect zgeo.Rect, start, end time.Time, fullLabelFirst bool, beyond bool, dark bool) {
	oldDay := -1
	oldHour := -1
	oldMin := -1
	ti := ztextinfo.New()
	ti.Font = zgeo.FontNice(12, zgeo.FontStyleNormal)
	ti.Alignment = zgeo.TopLeft

	col := zgeo.ColorBlack
	if dark {
		col = zgeo.ColorLightGray
	}
	inc, labelFieldStep, fullLabelFieldStep, t := ztime.NiceAxisIncrements(start, end, int(rect.Size.W))
	// var labelTime time.Time
	// zlog.Info("INC:", inc, incScale, incStep)
	endDraw := end
	if beyond {
		// t = t.Add(-inc)
		// endDraw = endDraw.Add(inc)
	}
	// drawTicks(canvas, rect, t, end, inc)
	y := rect.Max().Y
	endTextX := -1000.0
	// labelTime = t
	count := 0

	for !t.After(endDraw.Add(inc * 10)) {
		count++
		if count > 10000 { // sanity test
			zlog.Info("Break")
			return
		}
		x := TimeToX(rect, t, start, end)
		if x >= rect.Max().X {
			break
		}
		isLabel := labelFieldStep.IsModOfTimeZero(t)
		w := 1.0
		var fullLabel bool
		if fullLabelFirst {
			fullLabel = (t.Day() != oldDay)
		} else {
			fullLabel = fullLabelFieldStep.IsModOfTimeZero(t)
		}
		strokeCol := col
		if fullLabel {
			w = 2
		} else if !isLabel {
			strokeCol = strokeCol.WithOpacity(0.3)
		}
		canvas.SetColor(strokeCol)
		canvas.StrokeVertical(x, y-7, y, w, zgeo.PathLineSquare)
		secs := (inc <= time.Second*10)
		if x < endTextX+8 {
			t = t.Add(inc)
			continue
		}
		var str string
		if fullLabel {
			secs = (fullLabelFieldStep.Field == ztime.TimeFieldSecs)
			str = ztime.GetNice(t, secs)
			oldDay = t.Day()
			oldHour = t.Hour()
			oldMin = t.Minute()
		} else {
			if !isLabel {
				t = t.Add(inc)
				continue
			}
			var f []string
			if t.Hour() != oldHour {
				if oldHour != -1 {
					f = append(f, "15")
				}
				oldHour = t.Hour()
			}
			if t.Minute() != oldMin {
				if oldMin != -1 {
					str := "04"
					if len(f) == 0 {
						str = "04m"
					}
					f = append(f, str)
				}
				oldMin = t.Minute()
			}
			if secs {
				str := "05"
				if len(f) == 0 {
					str = "05s"
				}
				f = append(f, str)
			}
			str = t.Format(strings.Join(f, ":"))
		}
		ti.Color = col
		if fullLabel {
			ti.Color = col.MixedColor(zgeo.ColorCyan, 0.5)
		}
		ti.Text = str
		pos := zgeo.PosD(x-10, 4)
		// if fullLabel {
		// 	zlog.Info("FullLabel", secs, fullLabelFieldStep, t, x, endTextX, str)
		// }
		ti.Rect = zgeo.Rect{Pos: pos, Size: zgeo.SizeD(500, 20)}
		// s, _, _ := ti.GetBounds()
		// r := zgeo.RectFromCenterSize(pos, s)
		// if r.Pos.X > endTextX+10 {
		endTextX = ti.Draw(canvas).Max().X
		// }
		t = t.Add(inc)
	}
}

type GraphType string

const (
	GraphTypeBar  GraphType = "bar"
	GraphTypeLine GraphType = "line"
)

type AxisInfo struct {
	ValueRange        zmath.Range[float64]
	LineColor         zgeo.Color
	TextColor         zgeo.Color
	StrokeWidth       float64
	Font              *zgeo.Font
	LabelAlign        zgeo.Alignment
	SignificantDigits int
	Postfix           string
}

type GraphRow struct {
	Axis           AxisInfo
	Type           GraphType
	Start          time.Time
	End            time.Time
	Width          float64
	GraphColor     zgeo.Color
	PullValuesFunc func(i *int) (val, x float64, done bool)
}

func MakeAxisInfo() AxisInfo {
	return AxisInfo{
		TextColor:         zstyle.DefaultFGColor(),
		LineColor:         zstyle.DefaultFGColor().WithOpacity(0.2),
		StrokeWidth:       1,
		Font:              zgeo.FontDefault(-2),
		LabelAlign:        zgeo.Left,
		SignificantDigits: 1,
	}
}

func MakeGraphRow() GraphRow {
	return GraphRow{
		Width:      2,
		GraphColor: zstyle.DefaultFGColor(),
		Axis:       MakeAxisInfo(),
	}
}

func DrawBackgroundHorGraphLines(a *AxisInfo, rect zgeo.Rect, canvas *zcanvas.Canvas) {
	const lines = 6
	// zlog.Info("DrawBackgroundHorGraphLines:", rect, a)
	y0, inc := zmath.NiceDividesOf(a.ValueRange.Min, a.ValueRange.Max, lines, nil)
	// zlog.Info("NICEDIVS:", y0, inc, "for", a.ValueRange.Min, a.ValueRange.Max, lines)
	y1 := zmath.RoundUpToModF64(a.ValueRange.Max, inc)
	a.Font.Size = min(10, math.Floor((rect.Size.H+2)*2/lines))
	yScale := (y1 - y0) / rect.Size.H
	// zlog.Info("DrawGraphRow1", y0, y1, yScale, rect.Size.H)
	ti := ztextinfo.New()
	ti.Rect = rect.Expanded(zgeo.SizeD(-3, 0))
	ti.Font = a.Font
	ti.Color = a.TextColor
	for y := y0 + inc; y < y1; y += inc {
		var lastX = math.MaxFloat64
		pixy := rect.Max().Y - (y-y0)/yScale
		ti.Rect.Pos.Y = pixy - a.Font.Size/2
		ti.Rect.Size.H = a.Font.Size
		// zlog.Info("DrawGraphRow Y", y, y-y0, (y-y0)/vdiff, (y-y0)/vdiff*rect.Size.H, pixy)
		for _, align := range []zgeo.Alignment{zgeo.Left | zgeo.HorCenter, zgeo.Right} {
			if a.LabelAlign&align == 0 {
				continue
			}
			ti.Alignment = zgeo.VertCenter | align
			ti.Text = zwords.NiceFloat(y, a.SignificantDigits) + a.Postfix
			box := ti.Draw(canvas)
			// zlog.Info("DrawGraphRow", ti.Color, ti.Text)
			if lastX != math.MaxFloat64 {
				canvas.SetColor(a.LineColor) // We have to set this each time, as ti.Draw() above with set it too
				canvas.StrokeHorizontal(lastX, box.Min().X, pixy, a.StrokeWidth, zgeo.PathLineButt)
			}
			lastX = box.Max().X
		}
		if a.LabelAlign&zgeo.Right == 0 {
			canvas.SetColor(a.LineColor)
			canvas.StrokeHorizontal(lastX, rect.Max().X, pixy, a.StrokeWidth, zgeo.PathLineButt)
		}
	}
}

func DrawGraphRow(gr *GraphRow, rect zgeo.Rect, canvas *zcanvas.Canvas) {
	canvas.SetColor(gr.GraphColor)
	i := 0
	y0, inc := zmath.NiceDividesOf(gr.Axis.ValueRange.Min, gr.Axis.ValueRange.Max, 6, nil)
	y1 := zmath.RoundUpToModF64(gr.Axis.ValueRange.Max, inc)
	yScale := (y1 - y0) / rect.Size.H
	// zlog.Info("NICEDIVS:", y0, inc, "for", gr.Min, gr.Max, 6)
	path := zgeo.PathNew()
	for {
		val, x, done := gr.PullValuesFunc(&i)
		if done {
			break
		}
		pixy := rect.Max().Y - (val-y0)/yScale
		switch gr.Type {
		case GraphTypeLine:
			pos := zgeo.PosD(x, pixy)
			if path.IsEmpty() {
				path.MoveTo(pos)
			} else {
				path.LineTo(pos)
			}
		case GraphTypeBar:
			r := zgeo.RectFromXYWH(x-gr.Width, pixy, gr.Width, rect.Max().Y-2)
			path.Empty()
			path.AddRect(r, zgeo.SizeBoth(2))
			canvas.FillPath(path)
		}
	}
	switch gr.Type {
	case GraphTypeLine:
		canvas.StrokePath(path, gr.Width, zgeo.PathLineButt)
	}
}
