package zdraw

import (
	"math"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zmath/zhistogram"
)

type HistDrawOpts struct {
	MaxClassIndex int
	OutlierBelow  zbool.BoolInd
	OutlierAbove  zbool.BoolInd
	Styling       zstyle.Styling
	PercentCutoff int // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
	Symbol        string
	BarValueFunc  func(v float64) string
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
		(*bars) += 2 // we add 2 since we have a bar-gap to outliers
	}
	return draw
}

func DrawHistogram(h *zhistogram.Histogram, canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts) {
	classCount := len(h.Classes)
	if opts.MaxClassIndex != 0 {
		zint.Minimize(&classCount, opts.MaxClassIndex)
	}
	bars := classCount
	var totalCount int
	for i := range bars {
		totalCount += h.Classes[i]
	}
	drawBelow := shouldDrawOutlier(&bars, &totalCount, opts.OutlierBelow, h.OutlierBelow)
	drawAbove := shouldDrawOutlier(&bars, &totalCount, opts.OutlierAbove, h.OutlierAbove)

	barThickness := float64(int(rect.Size.W) / bars)

	x := math.Floor(rect.Size.W - barThickness*float64(bars))
	r := zgeo.RectFromXYWH(x, 0, float64(barThickness), rect.Size.H)
	if drawBelow {
		drawBar(canvas, r, opts, "", h.OutlierBelow, totalCount)
		r.Pos.X += barThickness * 2
	}
	barVal := h.Range.Min
	for i := range classCount {
		var str string
		barVal += h.Step
		if opts.BarValueFunc != nil {
			str = opts.BarValueFunc(barVal)
		}
		drawBar(canvas, r, opts, str, h.Classes[i], totalCount)
		r.Pos.X += barThickness
	}
	if drawAbove {
		r.Pos.X += barThickness
		drawBar(canvas, r, opts, "", h.OutlierAbove, totalCount)
	}
}

func drawBar(canvas *zcanvas.Canvas, rect zgeo.Rect, opts HistDrawOpts, label string, count, total int) {
	ti := ztextinfo.New()
	ti.Font = &opts.Styling.Font
	ti.Rect = rect
	ti.Rect.SetMinY(rect.Size.H - ti.Font.Size)
	ti.Alignment = zgeo.Center
	ti.Color = opts.Styling.FGColor
	ti.Text = label
	ti.Draw(canvas)

	rect.Size.H -= ti.Font.Size

	percent := 100 * count / total
	max := 100
	if opts.PercentCutoff != 0 {
		max = opts.PercentCutoff
	}
	h := float64(percent * int(rect.Size.H) / max)
	rect.SetMinY(rect.Size.H - h)
	canvas.SetColor(opts.Styling.BGColor)
}
