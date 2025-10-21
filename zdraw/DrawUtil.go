package zdraw

import (
	"image/color"
	"math"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/zwords"
)

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
	DrawAxisLine      zgeo.VerticeFlag
	Postfix           string
}

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

// DrawHorTimeAxis draws time labels with vertical ticks and an optional horizonal axis line.
func DrawHorTimeAxis(canvas zcanvas.BaseCanvaser, rect zgeo.Rect, start, end time.Time, beyond, isBottom, drawAxis bool, col, roundCol zgeo.Color, font *zgeo.Font) ztime.FieldInc {
	minLabelDist := font.Size * 2
	inc, labelInc, axisStart := ztime.NiceAxisIncrements(start, end, int(rect.Size.W), int(minLabelDist))
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
	axisY := y // + 1
	if isBottom {
		y = rect.Min().Y
		lineH -= 2
		axisY = y // - 1
	}
	endTextX := -1000.0
	count := 0
	// firstLabel := true
	canvas.SetFont(font, nil)
	prevDay := -1
	// zlog.Warn("Round:", axisStart, roundField, labelInc, inc)
	first := true
	for ot := axisStart; !ot.After(end.Add(inc.Duration() * time.Duration(10))); {
		nextRoundTime := ztime.OnThisPeriod(ot, roundField, 1)
		round := roundField == ztime.TimeFieldMonths
		for t := ot; t.Before(nextRoundTime); {
			count++
			if count > 10000 { // sanity test
				zlog.Warn("Break")
				return ztime.FieldInc{}
			}
			x := TimeToX(rect, t, start, end)
			if x >= rect.Max().X+200 {
				break
			}
			nextTime := ztime.OnThisPeriod(t, inc.Field, inc.Step)
			if x < rect.Min().X {
				t = nextTime
				continue
			}
			isLabel := round || labelInc.IsModOfTimeZero(t)
			if roundField.IsTimeZeroOfField(t) {
				round = true
			}
			strokeCol := col
			textOverlap := (x < endTextX)
			w := 1.0
			// strokeCol = strokeCol.WithOpacity(0.9)
			firstLabel := first && round && x >= rect.Min().X+30
			first = false
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
					if firstLabel {
						str = t.Format("2006-Jan-02 ")
					} else {
						str = t.Format("Jan-02")
					}
				} else {
					skip := false
					if firstLabel || day != prevDay {
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
			ty := rect.Max().Y - font.Size/3
			if !isBottom {
				ty = rect.Max().Y - lineH*1.2 - 2
			}
			pos := zgeo.PosD(x, ty)
			r := canvas.DrawTextAlignedInPos(pos, str, 0, zgeo.Center, 0)
			endTextX = r.Max
			t = nextTime
			round = false
		}
		ot = nextRoundTime
	}
	if drawAxis {
		canvas.SetColor(col)
		canvas.StrokeHorizontal(rect.Min().X, rect.Max().X, axisY, 1, zgeo.PathLineButt)
	}
	return inc
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

func DrawBackgroundHorGraphLines(canvas zcanvas.BaseCanvaser, a *AxisInfo, rect zgeo.Rect, gutter float64, lines int) {
	y0, inc := zmath.NiceDividesOf(a.ValueRange.Min, a.ValueRange.Max, lines, nil)
	y1 := a.ValueRange.Max
	a.Font.Size = min(10, math.Floor((rect.Size.H+2)*2/float64(lines)))
	canvas.SetFont(a.Font, nil)

	if a.DrawAxisLine.Vertical {
		canvas.SetColor(a.LineColor)
		canvas.StrokeVertical(rect.Min().X+gutter, rect.Min().Y, rect.Max().Y, 1, zgeo.PathLineButt)
	}
	for y := y0 + inc; y < y1; y += inc {
		var lastX = math.MaxFloat64
		pixy := ValToY(y, rect.Size.H, a.ValueRange)
		tx := 0.0
		if a.TextColor.Valid {
			pos := zgeo.PosD(tx, pixy+a.Font.Size/3)
			for _, align := range []zgeo.Alignment{zgeo.Left, zgeo.Right} {
				canvas.SetColor(a.TextColor)
				if a.LabelAlign&align == 0 {
					continue
				}
				align := zgeo.VertCenter | align
				text := zwords.NiceFloat(y, 0) + a.Postfix
				// zlog.Info("DrawLeft:", pos, text, align, rect)
				textRange := canvas.DrawTextAlignedInPos(pos, text, 0, align, 0)
				if a.LineColor.Valid && lastX != math.MaxFloat64 {
					canvas.SetColor(a.LineColor) // We have to set this each time, as ti.Draw() above with set it too
					canvas.StrokeHorizontal(lastX, textRange.Min, pixy, a.StrokeWidth, zgeo.PathLineButt)
				}
				lastX = textRange.Max + 3
			}
			tx = lastX
		}
		// zlog.Info("DrawGraphLine", y, inc, y1, rect)
		if a.LineColor.Valid && (!a.TextColor.Valid || !a.LabelAlign.Has(zgeo.Right)) {
			canvas.SetColor(a.LineColor)
			canvas.StrokeHorizontal(tx, rect.Max().X, pixy, a.StrokeWidth, zgeo.PathLineButt)
		}
	}
}
