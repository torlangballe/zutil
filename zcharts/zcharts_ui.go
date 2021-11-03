// +build zui
package zcharts

import (
	"time"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/ztime"
)

type Measure string

const (
	Time     Measure = "time"
	Duration Measure = "duration"
	KWH      Measure = "kwh"
	USD      Measure = "usd"
	NOK      Measure = "nok"
)

func ValueT(t time.Time, y float64) zgeo.Pos {
	return zgeo.Pos{X: XFromT(t), Y: y}
}

func XFromT(t time.Time) float64 {
	return float64(t.UnixMicro())
}

func TFromX(x float64) time.Time {
	return time.UnixMicro(int64(x))
}

type Chart struct {
	Values    []zgeo.Pos
	Color     zgeo.Color
	LineWidth float64
	MeasureX  Measure
	MeasureY  Measure
}

type ChartGroup struct {
	Title  string
	Charts []Chart
}

type ChartsDrawer struct {
	DrawVertLine  bool
	DrawHorLine   bool
	TickWidth     float32
	LineAlignment zgeo.Alignment
	TickLength    float64
	XIsTime       bool
	toRect        zgeo.Rect
	fromRect      zgeo.Rect
	Canvas        *zui.Canvas
	Font          *zgeo.Font
}

func NewCharsDrawer(canvas *zui.Canvas, toRect zgeo.Rect) *ChartsDrawer {
	c := &ChartsDrawer{}
	c.Canvas = canvas
	c.toRect = toRect
	c.TickWidth = 1
	c.DrawVertLine = true
	c.DrawHorLine = true
	c.Font = zgeo.FontDefault()
	c.LineAlignment = zgeo.Left | zgeo.Bottom
	c.TickLength = 10
	return c
}

func (c *ChartsDrawer) setupExtremes(chartGroups []ChartGroup) {
	for _, g := range chartGroups {
		for _, ch := range g.Charts {
			for _, v := range ch.Values {
				c.fromRect.UnionWithPos(v)
			}
		}
	}
}

func (c *ChartsDrawer) Draw(chartGroups []ChartGroup) {
	c.setupExtremes(chartGroups)
}

func (c *ChartsDrawer) getTicks(vertical bool) map[float64]string {
	m := map[float64]string{}
	min := c.fromRect.Min().Vertice(vertical)
	max := c.fromRect.Max().Vertice(vertical)
	if !vertical && c.XIsTime {
		var oldTime time.Time
		minT := TFromX(min)
		maxT := TFromX(max)
		parts := int(c.ToRect.Size.H / 10)
		inc, t := ztime.GetNiceIncsOf(minT, maxT, parts)
		for maxT.Sub(t) >= 0 {
			x := XFromT(t)
			t = t.Add(inc)
			secs := (inc < time.Minute)
			var str string
			if t.Day() != oldTime.Day() {
				str = ztime.GetNice(t, secs)
			} else {
				f := "15:04"
				if secs {
					f += ":05"
				}
				str = t.Format(f)
			}
			m[x] = str
			// zlog.Info("time:", str, x, rect.Max().Y-18)
			oldTime = t
		}
	} else {
		// math.GetNiceDividesOf(d float64, max int, isMemory bool) float64 {
	}
	return m
}

func (c *ChartsDrawer) drawAxis(vertical bool) {
	path := zgeo.PathNew()
	pos := c.toRect.Pos
	path.MoveTo(pos)
	if c.vertical {
		pos.Y = c.toRect.Max().Y
	} else {
		pos.X = c.toRect.Max().X
	}
	c.canvas.SetColor(c.LineColor)
	c.canvas.StrokePath(path, c.LineWidth, zgeo.PathLineSquare)

	ti := zui.TextInfoNew()
	ti.Font = c.Font
	if vertical {
		ti.Alignment = zgeo.TopCenter
	} else {
		ti.Alignment = zgeo.CenterRight
	}
	ti.Color = c.LineColor
	edge := c.fromRect.Align(zgeo.Size{}, c.LineAlignment)
	for tick := range c.getTicks(vertical) {
		var pos zgeo.Pos
		*pos.VerticeP(vertical) = tick.Pos().Vertice[vertical]
		*pos.VerticeP(!vertical) = edge.Vertice(!vertical)
		tpos := c.mapPos(pos)
		ti.Text = tick.String()
		if vertical {
			if c.LineAlignment&zgeo.Left != 0 {
				canvas.StrokeHorizontal(tpos.X, tpos.X-c.TickLength, tpos.Y, c.LineWidth, zgeo.PathLineSquare)
				ti.Rect = zgeo.RectFromXY2(tpos.X-200, tpos.Y-20, tpos.X-10, tpos.Y+20)
			} else {
				canvas.StrokeHorizontal(tpos.X, tpos.X+c.TickLength, tpos.Y, c.LineWidth, zgeo.PathLineSquare)
				ti.Rect = zgeo.RectFromXY2(tpos.X-200, tpos.Y-20, tpos.X-10, tpos.Y+20)
			}
		} else {
			ti.Rect = zgeo.RectFromXY2(tpos.X-100, tpos.Y+10, tpos.X+100, tpos.Y+40)
		}
		ti.Draw(canvas)
	}
}

/*
func DrawTimeAxisesOld(canvas *zui.Canvas, rect zgeo.Rect, start, end time.Time, lineColor zgeo.Color, below bool, font *zgeo.Font) {
	drawTicks(canvas, rect, start, end)
	parts := int(rect.Size.W / 150)
	var oldTime time.Time
	ti := zui.TextInfoNew()
	ti.Font = font
	ti.Alignment = zgeo.TopCenter

	inc, t := ztime.GetNiceIncsOf(start, end, parts)
	// zlog.Info("niceincs:", inc, t, start, end, parts, rect)
	y := rect.Max().Y
	for end.Sub(t) >= 0 {
		x := TimeToX(rect, t, start, end)
		if x >= rect.Max().X {
			break
		}
		canvas.SetColor(zgeo.ColorBlack)
		if below {

		}
		canvas.StrokeVertical(x, y-7, y, 1, zgeo.PathLineSquare)
		secs := (inc < time.Minute)
		var str string
		if t.Day() != oldTime.Day() {
			str = ztime.GetNice(t, secs)
		} else {
			f := "15:04"
			if secs {
				f += ":05"
			}
			str = t.Format(f)
		}
		// zlog.Info("time:", str, x, rect.Max().Y-18)
		oldTime = t
		ti.Text = str
		pos := zgeo.Pos{x, rect.Max().Y - 10}
		ti.Rect = zgeo.RectFromCenterSize(pos, zgeo.Size{300, 20})
		ti.Color = zgeo.ColorBlack
		ti.Draw(canvas)
		t = t.Add(inc)
	}
	// zlog.Info("draw timelines:", popupTime)
}

*/
