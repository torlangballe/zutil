package zdraw

import (
	"fmt"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zmath/zhistogram"
	"github.com/torlangballe/zutil/zwords"
)

type HistDrawOpts struct {
	MaxClassIndex      int
	OutlierBelow       zbool.BoolInd
	OutlierAbove       zbool.BoolInd
	Styling            zstyle.Styling
	CriticalClassValue float64 // if a class bar has value >= this, show in red
	PercentCutoff      int     // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
	SignificantDigits  int     // For bar-bottom labels
	BarValueFunc       func(v float64) string
}

func DrawHistogram(h *zhistogram.Histogram, canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts) {
	// canvas.SetColor(zgeo.ColorBlack)
	// canvas.FillRect(rect)

	rect.Add(opts.Styling.Margin)

	// canvas.SetColor(zgeo.ColorGreen)
	// canvas.FillRect(rect)

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
		totalCount += h.Classes[i]
	}
	// zlog.Warn("Draw:", opts.OutlierBelow)

	drawBelow := shouldDrawOutlier(&bars, opts.OutlierBelow, h.OutlierBelow)
	drawAbove := shouldDrawOutlier(&bars, opts.OutlierAbove, h.OutlierAbove)

	barThickness, x := zmath.CellSizeInWidthF(rect.Size.W, opts.Styling.Spacing, 0, 0, bars) // we call without margins, as rect has them removed already
	x += rect.Pos.X
	barAdd := barThickness + opts.Styling.Spacing
	r := zgeo.RectFromXYWH(x, 0, float64(barThickness), rect.Size.H)
	if drawBelow && h.OutlierBelow != 0 {
		drawBar(h, canvas, r, opts, "<", h.OutlierBelow, true, false)
		r.Pos.X += barAdd
	}
	barVal := h.Range.Min
	for i := range classCount {
		var str string
		barVal += h.Step
		// zlog.Warn("barVal:", barVal, h.Step)
		sig := 7 // we do this so it doesn't do weird things like be 0.1, 0.2, 0.300000000001, 0.4 etc
		if opts.SignificantDigits != 0 {
			sig = opts.SignificantDigits + 1 // need to add 1 or we round down any changes
		}
		barVal = zfloat.KeepFractionDigits(barVal, sig)
		val := barVal
		if opts.BarValueFunc != nil {
			str = opts.BarValueFunc(barVal)
		} else {
			if opts.SignificantDigits == 0 {
				str = fmt.Sprint(int(val))
			} else {
				str = zwords.NiceFloat(val, opts.SignificantDigits)
			}
			// zlog.Warn("Label:", len(h.Classes), str, val, barVal)
		}
		critical := opts.CriticalClassValue != 0 && barVal > opts.CriticalClassValue
		// zlog.Warn("CRIT:", critical, barVal, opts.CriticalClassValue)
		drawBar(h, canvas, r, opts, str, h.Classes[i], false, critical)
		r.Pos.X += barAdd
	}
	if drawAbove && h.OutlierAbove != 0 {
		// zlog.Info("DrawOutlierAbove:", h.OutlierAbove)
		drawBar(h, canvas, r, opts, ">", h.OutlierAbove, true, false)
	}
}

func drawBar(h *zhistogram.Histogram, canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts, label string, count int, isOutlier, isCritical bool) {
	labelBoxHeight := opts.Styling.Font.Size * 1.5
	ti := ztextinfo.New()
	ti.Font = &opts.Styling.Font
	ti.Rect = rect
	ti.Rect.SetMinY(rect.Size.H - labelBoxHeight + opts.Styling.Font.Size/3)
	ti.Alignment = zgeo.Center
	ti.Color = opts.Styling.FGColor
	ti.Text = label
	ti.Draw(canvas)

	// zlog.Warn("DrawBar:", rect, label, count, total)
	rect.Size.H -= labelBoxHeight
	ratio := h.CountAsRatio(count)
	max := 1.0
	if opts.PercentCutoff != 0 {
		max = float64(opts.PercentCutoff) / 100
	}
	y := ratio * rect.Size.H / max
	rect.SetMinY(rect.Size.H - y)
	path := zgeo.PathNewRect(rect, zgeo.SizeBoth(opts.Styling.Corner))
	col := opts.Styling.BGColor
	if isOutlier {
		col = col.Mixed(zgeo.ColorRed, 0.5)
	}
	if isCritical {
		col = zgeo.ColorRed
	}
	canvas.SetColor(col)
	canvas.FillPath(path)
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
