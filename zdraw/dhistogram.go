package zdraw

import (
	"fmt"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zmath/zhistogram"
	"github.com/torlangballe/zutil/zwords"
)

type HistDrawOpts struct {
	MaxClassIndex      int
	OutlierBelow       zbool.BoolInd
	OutlierAbove       zbool.BoolInd
	Styling            zstyle.Styling
	PercentCutoff      int // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
	SignificantDigits  int
	ScaleClassValue    float64
	CriticalClassValue float64
	BarValueFunc       func(v float64) string
}

func DrawHistogram(h *zhistogram.Histogram, canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts) {
	canvas.SetColor(zgeo.ColorBlack)
	canvas.FillRect(rect)

	rect.Add(opts.Styling.Margin)

	// canvas.SetColor(zgeo.ColorGreen)
	// canvas.FillRect(rect)

	classCount := len(h.Classes)
	if opts.MaxClassIndex != 0 {
		zint.Minimize(&classCount, opts.MaxClassIndex)
	}
	bars := classCount
	var totalCount int
	for i := range bars {
		totalCount += h.Classes[i]
	}
	zlog.Warn("Draw:", opts.OutlierBelow)

	drawBelow := shouldDrawOutlier(&bars, &totalCount, opts.OutlierBelow, h.OutlierBelow)
	drawAbove := shouldDrawOutlier(&bars, &totalCount, opts.OutlierAbove, h.OutlierAbove)

	if totalCount == 0 {
		return
	}
	barThickness, x := zmath.CellSizeInWidthF(rect.Size.W, opts.Styling.Spacing, 0, 0, bars) // we call without margins, as rect has them removed already
	x += rect.Pos.X
	barAdd := barThickness + opts.Styling.Spacing
	r := zgeo.RectFromXYWH(x, 0, float64(barThickness), rect.Size.H)
	if drawBelow {
		drawBar(canvas, r, opts, "<", h.OutlierBelow, totalCount, true)
		r.Pos.X += barAdd
	}
	barVal := h.Range.Min
	for i := range classCount {
		var str string
		barVal += h.Step
		val := barVal
		if opts.BarValueFunc != nil {
			str = opts.BarValueFunc(barVal)
		} else {
			if opts.ScaleClassValue != 0 {
				val *= opts.ScaleClassValue
			}
			if opts.SignificantDigits == 0 {
				str = fmt.Sprint(int(val))
			} else {
				str = zwords.NiceFloat(val, opts.SignificantDigits)
			}
			// zlog.Warn("Label:", str)
		}
		drawBar(canvas, r, opts, str, h.Classes[i], totalCount, false)
		r.Pos.X += barAdd
	}
	if drawAbove {
		drawBar(canvas, r, opts, ">", h.OutlierAbove, totalCount, true)
	}
}

func drawBar(canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts, label string, count, total int, isOutlier bool) {
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
	percent := 100 * count / total
	max := 100
	if opts.PercentCutoff != 0 {
		max = opts.PercentCutoff
	}
	h := float64(percent * int(rect.Size.H) / max)
	rect.SetMinY(rect.Size.H - h)
	path := zgeo.PathNewRect(rect, zgeo.SizeBoth(opts.Styling.Corner))
	col := opts.Styling.BGColor
	if isOutlier {
		col = col.Mixed(zgeo.ColorBlack, 0.5)
	}
	canvas.SetColor(col)
	canvas.FillPath(path)
}

func shouldDrawOutlier(bars *int, totalCount *int, want zbool.BoolInd, outCount int) bool {
	var draw bool
	if !want.IsFalse() {
		if want.IsTrue() {
			draw = true
		} else {
			draw = outCount != 0
		}
		*totalCount += outCount
	}
	if draw {
		(*bars)++
	}
	return draw
}
