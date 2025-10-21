package zdraw

import (
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
)

type Graph struct {
	Axis           AxisInfo
	Type           GraphType
	Start          time.Time
	End            time.Time
	StrokeWidth    float64
	GraphColor     zgeo.Color
	PullValuesFunc func(i *int) (val float64, x time.Time)
	AxisSizes      zgeo.Size // x is actually for y axis and visa versa
}

func DrawGraph(canvas zcanvas.BaseCanvaser, gr *Graph, fullRect zgeo.Rect) {
	canvas.SetColor(gr.GraphColor)
	i := 0

	if gr.AxisSizes.IsNull() {
		gr.AxisSizes.W = 40
		gr.AxisSizes.H = gr.Axis.Font.Size * 2
	}
	yAxisRect := fullRect
	xAxisRect := fullRect
	yAxisRect.Size.W = gr.AxisSizes.W
	xAxisRect.SetMinY(xAxisRect.Max().Y - gr.AxisSizes.H)
	xAxisRect.SetMinX(yAxisRect.Max().X)
	yAxisRect.SetMaxY(xAxisRect.Min().Y)
	rect := fullRect
	rect.SetMinX(yAxisRect.Max().X)
	rect.Size.H -= xAxisRect.Size.H

	y0, inc := zmath.NiceDividesOf(gr.Axis.ValueRange.Min, gr.Axis.ValueRange.Max, 6, nil)
	y1 := zmath.RoundUpToModF64(gr.Axis.ValueRange.Max, inc)
	yScale := (y1 - y0) / rect.Size.H
	path := zgeo.PathNew()

	for {
		val, xTime := gr.PullValuesFunc(&i)
		if i == -1 {
			break
		}
		x := TimeToX(rect, xTime, gr.Start, gr.End)
		pixy := rect.Max().Y - (val-y0)/yScale
		switch gr.Type {
		case GraphTypeLine:
			pos := zgeo.PosD(x, pixy)
			// zlog.Warn("Draw:", pos, gr.GraphColor)
			if path.IsEmpty() {
				path.MoveTo(pos)
			} else {
				path.LineTo(pos)
			}
		case GraphTypeBar:
			r := zgeo.RectFromXYWH(x-gr.StrokeWidth, pixy, gr.StrokeWidth, rect.Max().Y-2)
			canvas.FillRect(r, 2)
		default:
			zlog.Fatal("Bad type:", gr.Type)
		}
	}
	switch gr.Type {
	case GraphTypeLine:
		canvas.StrokePath(path, gr.StrokeWidth, zgeo.PathLineButt)
	}
	isBottom := true
	drawAxis := gr.Axis.DrawAxisLine.Horizontal
	beyond := false
	DrawHorTimeAxis(canvas, xAxisRect, gr.Start, gr.End, beyond, isBottom, drawAxis, gr.Axis.LineColor, gr.GraphColor, gr.Axis.Font)
	horRect := fullRect
	horRect.SetMaxY(xAxisRect.Min().Y)
	DrawBackgroundHorGraphLines(canvas, &gr.Axis, horRect, yAxisRect.Size.W, 5)
}

func MakeGraphInfo() Graph {
	return Graph{
		Type:        GraphTypeLine,
		StrokeWidth: 2,
		GraphColor:  zstyle.DefaultFGColor(),
		Axis:        MakeAxisInfo(),
	}
}
