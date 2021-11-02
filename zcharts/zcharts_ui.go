// +build zui
package zcharts

import (
	"time"

	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/ztime"
)

// func (v *GraphView) Render() error {
// 	var buffer bytes.Buffer
// 	err := v.Renderer.Render(&buffer)
// 	if err != nil {
// 		return err
// 	}
// 	v.SetHTMLContent(string(buffer.Bytes()))
// 	return nil
// }

type Measure string

const (
	Duration Measure = "duration"
	KWH      Measure = "kwh"
	Money    Measure = "money"
)

type Value struct {
	Y float64
	X float64
	T time.Time
}

type Chart struct {
	Values []Value
	Color  zgeo.Color
	Width  float64
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
	toRect        zgeo.Rect
	minX          float64
	maxX          float64
	minY          float64
	maxY          float64
	minTime       time.Time
	maxTime       time.Time
	canvas        *zui.Canvas
	Font          *zgeo.Font
}

func NewCharsDrawer(canvas *zui.Canvas, toRect zgeo.Rect) *ChartsDrawer {
	c := &ChartsDrawer{}
	c.canvas = canvas
	c.toRect = toRect
	c.TickWidth = 1
	c.DrawVertLine = true
	c.DrawHorLine = true
	c.Font = zgeo.FontDefault()
	c.LineAlignment = zgeo.Left | zgeo.Bottom
	c.TickLength = 10
	return c
}

func (c *ChartsDrawer) setupExtremes(chartGroups []ChartGroup, xIsTime bool) {
	for _, g := range chartGroups {
		for _, ch := range g.Charts {
			for _, v := range ch.Values {
				if xIsTime {
					ztime.Minimize(&c.minTime, v.T)
					ztime.Maximize(&c.maxTime, v.T)
				} else {
					zfloat.Minimize(&c.minX, v.X)
					zfloat.Maximize(&c.maxX, v.Y)
				}
				zfloat.Minimize(&c.minY, v.X)
				zfloat.Maximize(&c.maxY, v.Y)
			}
		}
	}
}

func (c *ChartsDrawer) Draw(chartGroups []ChartGroup, xIsTime bool) {
	c.setupExtremes(chartGroups, xIsTime)
}

/*
func (c *ChartsDrawer) drawAxis(vertical bool, ticks []ValueStringer) {
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
	for tick := range ticks {
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
