package zhistogram

import (
	"fmt"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zwords"
)

type DrawOpts struct {
	MaxClassIndex      int
	OutlierBelow       zbool.BoolInd
	OutlierAbove       zbool.BoolInd
	Styling            zstyle.Styling
	CriticalClassValue float64                             // if a class bar has value >= this, show in red
	PercentCutoff      int                                 // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
	SignificantDigits  int                                 // For bar-bottom labels
	BarNameFunc        func(n string) (string, zgeo.Color) // this is for transforming named classes' names and getting a color for it, if Valid
}

func MakeDefaultStyling(size zgeo.Size) zstyle.Styling {
	min := size.Min()
	s := min / 12
	return zstyle.Styling{
		Font:    *zgeo.FontNew("Righteous-Regular", s, zgeo.FontStyleNormal),
		FGColor: zstyle.DefaultFGColor(),
		BGColor: zgeo.ColorGreen,
		Corner:  min / 50,
		Spacing: min / 40,
		Margin:  zgeo.RectFromMarginSize(zgeo.SizeBoth(min / 50)),
	}
}

func (h *Histogram) Draw(canvas zcanvas.BaseCanvaser, rect zgeo.Rect, opts DrawOpts) {
	rect.Add(opts.Styling.Margin)
	totalCount := h.TotalCount()
	if totalCount == 0 {
		return
	}
	classCount := len(h.Classes)
	if opts.MaxClassIndex != 0 {
		zint.Minimize(&classCount, opts.MaxClassIndex)
	}
	bars := classCount
	for i := range bars {
		totalCount += h.Classes[i].Count
	}

	drawBelow := shouldDrawOutlier(&bars, opts.OutlierBelow, h.OutlierBelow)
	drawAbove := shouldDrawOutlier(&bars, opts.OutlierAbove, h.OutlierAbove)

	barThickness, x := zmath.CellSizeInWidthF(rect.Size.W, opts.Styling.Spacing, 0, 0, bars) // we call without margins, as rect has them removed already
	x += rect.Pos.X
	barAdd := barThickness + opts.Styling.Spacing
	r := zgeo.RectFromXYWH(x, 0, float64(barThickness), rect.Size.H)
	if drawBelow && h.OutlierBelow != 0 {
		drawBar(h, canvas, r, opts, opts.Styling.BGColor, "<", h.OutlierBelow, true, false)
		r.Pos.X += barAdd
	}
	for i := range classCount {
		var str string
		var critical bool
		col := opts.Styling.BGColor
		class := h.Classes[i]
		str = class.Label
		if str != "" && opts.BarNameFunc != nil {
			var bcol zgeo.Color
			str, bcol = opts.BarNameFunc(str)
			if bcol.Valid {
				col = bcol
			}
		} else {
			barVal := class.MaxRange
			sig := 7 // we do this so it doesn't do weird things like be 0.1, 0.2, 0.300000000001, 0.4 etc
			if opts.SignificantDigits != 0 {
				sig = opts.SignificantDigits + 1 // need to add 1 or we round down any changes
			}
			barVal = zfloat.KeepFractionDigits(barVal, sig)
			if sig == 0 {
				str = fmt.Sprint(int(barVal))
			} else {
				str = zwords.NiceFloat(barVal, sig)
			}
			critical = opts.CriticalClassValue != 0 && barVal > opts.CriticalClassValue
		}
		// zlog.Warn("drawBar:", r, str, class.Count, rect)
		drawBar(h, canvas, r, opts, col, str, class.Count, false, critical)
		r.Pos.X += barAdd
	}
	if drawAbove && h.OutlierAbove != 0 {
		// zlog.Info("DrawOutlierAbove:", h.OutlierAbove)
		drawBar(h, canvas, r, opts, opts.Styling.BGColor, ">", h.OutlierAbove, true, false)
	}
}

func drawBar(h *Histogram, canvas zcanvas.BaseCanvaser, rect zgeo.Rect, opts DrawOpts, col zgeo.Color, label string, count int, isOutlier, isCritical bool) {
	labelBoxHeight := opts.Styling.Font.Size * 1
	canvas.SetFont(&opts.Styling.Font, nil)
	bottom := rect.Size.H - labelBoxHeight + opts.Styling.Font.Size/3 + 2
	pos := zgeo.PosD(rect.Center().X, rect.Max().Y+labelBoxHeight/3)

	canvas.SetColor(opts.Styling.FGColor)
	canvas.DrawTextAlignedInPos(pos, label, 0, zgeo.TopCenter)

	rect.Size.H = bottom - 2
	ratio := h.CountAsRatio(count)
	max := 1.0
	if opts.PercentCutoff != 0 {
		max = float64(opts.PercentCutoff) / 100
	}
	y := ratio * rect.Size.H / max
	rect.SetMinY(rect.Size.H - y)
	// path := zgeo.PathNewRect(rect, zgeo.SizeBoth(opts.Styling.Corner))
	// zlog.Warn("drawBar:", label, pos, col, isCritical, isOutlier, count, rect, max, opts.PercentCutoff)
	if isOutlier {
		col = col.Mixed(zgeo.ColorRed, 0.5)
	}
	if isCritical {
		col = zgeo.ColorRed
	}
	canvas.SetColor(col)
	canvas.FillRect(rect, 0)
}

func shouldDrawOutlier(bars *int, want zbool.BoolInd, outCount int) bool {
	var draw bool
	if !want.IsFalse() {
		if want.IsTrue() {
			draw = true
		} else {
			draw = outCount != 0
		}
	}
	if draw {
		(*bars)++
	}
	return draw
}
