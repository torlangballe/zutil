package zdraw

import (
	"image/color"
	"math"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zfloat"
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

func DrawHorTimeAxis(canvas *zcanvas.Canvas, rect zgeo.Rect, start, end time.Time, beyond, isBottom bool, col, roundCol zgeo.Color, font *zgeo.Font) {
	inc, labelInc, axisStart := ztime.NiceAxisIncrements(start, end, int(rect.Size.W))
	var roundField ztime.TimeFieldFlags
	switch labelInc.Field {
	case ztime.TimeFieldYears:
		roundField = ztime.TimeFieldYears
	case ztime.TimeFieldMonths:
		roundField = ztime.TimeFieldYears
		if labelInc.Step == 1 {
			roundField = ztime.TimeFieldMonths
		}
	case ztime.TimeFieldDays:
		roundField = ztime.TimeFieldMonths
	case ztime.TimeFieldHours:
		roundField = ztime.TimeFieldDays
	case ztime.TimeFieldMins:
		roundField = ztime.TimeFieldHours
	case ztime.TimeFieldSecs:
		roundField = ztime.TimeFieldMins
	}
	lineH := rect.Size.H - font.Size
	y := rect.Max().Y - lineH
	if isBottom {
		y = rect.Min().Y
	}
	endTextX := -1000.0
	count := 0
	// firstLabel := true
	canvas.SetFont(font, nil)
	prevDay := -1
	// zlog.Warn("Round:", axisStart, roundField, labelInc, inc)
	for ot := axisStart; !ot.After(end.Add(inc.Duration() * time.Duration(10))); {
		nextRoundTime := ztime.OnThisPeriod(ot, roundField, 1)
		round := roundField == ztime.TimeFieldMonths
		for t := ot; t.Before(nextRoundTime); {
			count++
			if count > 10000 { // sanity test
				zlog.Warn("Break")
				return
			}
			x := TimeToX(rect, t, start, end)
			if x >= rect.Max().X+200 {
				break
			}
			nextTime := ztime.OnThisPeriod(t, inc.Field, inc.Step)
			isLabel := round || labelInc.IsModOfTimeZero(t)
			if roundField.IsTimeZeroOfField(t) {
				round = true
			}
			strokeCol := col
			textOverlap := (x < endTextX)
			w := 1.0
			// strokeCol = strokeCol.WithOpacity(0.9)
			first := (round && beyond && x >= rect.Max().X-2) // firstLabel ||
			if isLabel {
				w = 2.0
			}
			if isLabel && round {
				// zlog.Warn("STROKECOL:", round, isLabel, x, t)
				strokeCol = roundCol
			}
			canvas.SetColor(strokeCol)
			canvas.StrokeVertical(x, y, y+lineH, w, zgeo.PathLineSquare)
			secs := (inc.Duration() < time.Second*10)
			nearRound := false //nextRoundTime.Sub(t) < inc.Duration()*3
			// zlog.Warn("IsLabel:", isLabel, t, labelInc, inc, round)
			var str string
			if textOverlap || !isLabel || nearRound || !beyond && (x < 30 || x > rect.Max().X-30) {
				round = false
				t = nextTime
				continue
			}
			day := t.Day()
			if labelInc.Field == ztime.TimeFieldYears {
				str = t.Format("2006")
			} else {
				if inc.Field >= ztime.TimeFieldDays {
					if first {
						str = t.Format("2006-Jan-02 ")
					} else {
						str = t.Format("Jan-02")
					}
				} else {
					skip := false
					if first || day != prevDay {
						str = t.Format("Jan-02 ")
						// zlog.Warn("SKIP?", str, t)
						if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
							skip = true
						}
					}
					if !skip {
						secs = (inc.Field == ztime.TimeFieldSecs)
						format := "15:04"
						if secs {
							format += ":05"
						}
						str += t.Format(format)
					}
				}
			}
			prevDay = day
			canvas.SetColor(col)
			// zlog.Warn("LABCOL:", round, str)
			if round {
				canvas.SetColor(roundCol)
			}
			ty := rect.Max().Y
			if !isBottom {
				ty -= lineH * 1.2
			}
			pos := zgeo.PosD(x, ty)
			r := canvas.DrawTextAlignedInPos(pos, str, 0, zgeo.Center)
			endTextX = r.Max
			t = nextTime
			round = false
		}
		ot = nextRoundTime
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

func ValToY(val float64, cellHeight float64, valRange zmath.RangeF64) float64 {
	y := cellHeight * ((val - valRange.Min) / valRange.Length())
	zfloat.Maximize(&y, 1)
	// y = math.Ceil(y)
	return cellHeight - y
}

func DrawBackgroundHorGraphLines(a *AxisInfo, rect zgeo.Rect, lines int, canvas *zcanvas.Canvas) {
	// zlog.Info("DrawBackgroundHorGraphLines:", rect, a)
	y0, inc := zmath.NiceDividesOf(a.ValueRange.Min, a.ValueRange.Max, lines, nil)
	// zlog.Info("NICEDIVS:", y0, inc, "for", a.ValueRange.Min, a.ValueRange.Max, lines)
	// y1 := zmath.RoundUpToModF64(a.ValueRange.Max, inc)
	y1 := a.ValueRange.Max
	a.Font.Size = min(10, math.Floor((rect.Size.H+2)*2/float64(lines)))
	// yScale := (y1 - a.ValueRange.Min) / rect.Size.H
	// zlog.Info("DrawGraphRow1", y0, y1, yScale, rect.Size.H)
	ti := ztextinfo.New()
	ti.Rect = rect.Expanded(zgeo.SizeD(-3, 0))
	ti.Font = a.Font
	ti.Color = a.TextColor
	for y := y0 + inc; y < y1; y += inc {
		var lastX = math.MaxFloat64
		pixy := ValToY(y, rect.Size.H, a.ValueRange)
		// rect.Max().Y - (y-a.ValueRange.Min)/yScale
		// pixy = math.Floor(pixy)
		tx := 0.0
		if a.TextColor.Valid {
			ti.Rect.Pos.Y = pixy - a.Font.Size/2
			ti.Rect.Size.H = a.Font.Size
			// zlog.Info("DrawGraphRow Y", y, y-y0, (y-y0)/vdiff, (y-y0)/vdiff*rect.Size.H, pixy)
			for _, align := range []zgeo.Alignment{zgeo.Left | zgeo.HorCenter, zgeo.Right} {
				if a.LabelAlign&align == 0 {
					continue
				}
				ti.Alignment = zgeo.VertCenter | align
				// ti.Text = zwords.NiceFloat(y, a.SignificantDigits) + a.Postfix
				ti.Text = zwords.NiceFloat(y, 0) + a.Postfix
				box := ti.Draw(canvas)
				if a.LineColor.Valid && lastX != math.MaxFloat64 {
					// zlog.Info("DrawGraphRow", a.LineColor)
					canvas.SetColor(a.LineColor) // We have to set this each time, as ti.Draw() above with set it too
					canvas.StrokeHorizontal(lastX, box.Min().X, pixy, a.StrokeWidth, zgeo.PathLineButt)
				}
				lastX = box.Max().X
			}
			tx = lastX
		}
		if a.LineColor.Valid && (!a.TextColor.Valid || !a.LabelAlign.Has(zgeo.Right)) {
			// zlog.Info("DrawGraphRow2", a.LineColor)
			canvas.SetColor(a.LineColor)
			canvas.StrokeHorizontal(tx, rect.Max().X, pixy, a.StrokeWidth, zgeo.PathLineButt)
		}
	}
}
