package zdraw

import (
	"image/color"
	"strings"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
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

// func drawTicks(canvas *zcanvas.Canvas, rect zgeo.Rect, start, end time.Time, inc time.Duration) {
// 	parts := int(rect.Size.W / 30)
// 	t := start
// 	if inc == 0 {
// 		inc, t = ztime.GetNiceIncsOf(start, end, parts)
// 	}
// 	// zlog.Info("niceincs:", inc, t, start, end, parts, rect)
// 	y1 := rect.Max().Y - 5
// 	y2 := rect.Max().Y
// 	col := zgeo.ColorNewGray(0, 0.2)
// 	for end.Sub(t) >= 0 {
// 		x := TimeToX(rect, t, start, end)
// 		t = t.Add(inc)
// 		canvas.SetColor(col)
// 		canvas.StrokeVertical(x, y1, y2, 1, zgeo.PathLineSquare)
// 	}
// }

func XToTime(rect zgeo.Rect, x float64, start, end time.Time) time.Time {
	tdiff := ztime.DurSeconds(end.Sub(start))
	dur := ztime.SecondsDur((x - rect.Pos.X) / rect.Size.W * tdiff)
	return start.Add(dur)
}

func TimeToX(rect zgeo.Rect, t, start, end time.Time) float64 {
	diff := ztime.DurSeconds(end.Sub(start))
	return rect.Min().X + ztime.DurSeconds(t.Sub(start))*rect.Size.W/diff
}

func DrawHorTimeAxis(canvas *zcanvas.Canvas, rect zgeo.Rect, start, end time.Time, beyond bool) {
	oldDay := -1
	oldHour := -1
	oldMin := -1
	parts := int(rect.Size.W / 20)
	ti := ztextinfo.New()
	ti.Font = zgeo.FontNice(12, zgeo.FontStyleNormal)
	ti.Alignment = zgeo.TopCenter

	inc, labelInc, t := ztime.GetNiceIncsOf(start, end, parts)
	labelStart := t
	endDraw := end
	if beyond {
		t = t.Add(-inc)
		endDraw = endDraw.Add(inc)
	}
	// drawTicks(canvas, rect, t, end, inc)
	// zlog.Info("niceincs:", inc, t, labelStart, "start:", start, end.Sub(start))
	y := rect.Max().Y
	for !t.After(endDraw) {
		x := TimeToX(rect, t, start, end)
		if x >= rect.Max().X {
			break
		}
		isLabel := (t == labelStart)
		// zlog.Info("tick:", t, isLabel)
		col := zgeo.ColorNewGray(0, 0.3)
		if isLabel {
			col = zgeo.ColorBlack
			labelStart = labelStart.Add(labelInc)
		}
		canvas.SetColor(col)
		canvas.StrokeVertical(x, y-7, y, 1, zgeo.PathLineSquare)
		// if isLabel {
		secs := (inc < time.Minute)
		var str string
		if x > rect.Min().X+30 && t.Day() != oldDay {
			str = ztime.GetNice(t, secs)
			oldDay = t.Day()
			oldHour = t.Hour()
			oldMin = t.Minute()
		} else {
			var f []string
			if isLabel && t.Hour() != oldHour {
				f = append(f, "15")
				oldHour = t.Hour()
			}
			if t.Minute() != oldMin {
				f = append(f, "04")
				oldMin = t.Minute()
			}
			if secs {
				f = append(f, "05")
			}
			str = t.Format(strings.Join(f, ":"))
		}
		// zlog.Info("time:", str, x, rect.Max().Y-18)
		ti.Text = str
		pos := zgeo.PosD(x, rect.Max().Y-9)
		ti.Rect = zgeo.RectFromCenterSize(pos, zgeo.SizeD(300, 20))
		ti.Color = col
		ti.Draw(canvas)
		// }
		t = t.Add(inc)
	}
}
