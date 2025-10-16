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

func DrawGraph(gr *Graph, fullRect zgeo.Rect, canvas *zcanvas.Canvas) {
	canvas.SetColor(gr.GraphColor)
	i := 0
	yAxisRect := fullRect
	yAxisRect.Size.W = gr.AxisSizes.W
	xAxisRect := fullRect
	xAxisRect.SetMinY(xAxisRect.Max().Y - gr.AxisSizes.H)
	rect := fullRect
	rect.IncMinX(gr.AxisSizes.W)
	rect.Size.H -= gr.AxisSizes.H
	y0, inc := zmath.NiceDividesOf(gr.Axis.ValueRange.Min, gr.Axis.ValueRange.Max, 6, nil)
	y1 := zmath.RoundUpToModF64(gr.Axis.ValueRange.Max, inc)
	yScale := (y1 - y0) / rect.Size.H
	// zlog.Info("NICEDIVS:", y0, y1, inc, gr.Axis.ValueRange)
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
			path.Empty()
			path.AddRect(r, zgeo.SizeBoth(2))
			canvas.FillPath(path)
		default:
			zlog.Fatal("Bad type:", gr.Type)
		}
	}
	switch gr.Type {
	case GraphTypeLine:
		canvas.StrokePath(path, gr.StrokeWidth, zgeo.PathLineButt)
	}
	isBottom := true
	DrawHorTimeAxis(canvas, xAxisRect, gr.Start, gr.End, false, isBottom, zgeo.ColorWhite, zgeo.ColorMagenta, gr.Axis.Font)
}

func MakeGraphInfo() Graph {
	return Graph{
		Type:        GraphTypeLine,
		StrokeWidth: 2,
		GraphColor:  zstyle.DefaultFGColor(),
		Axis:        MakeAxisInfo(),
		AxisSizes:   zgeo.SizeD(40, 20),
	}
}
