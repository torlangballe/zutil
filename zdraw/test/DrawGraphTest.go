package main

import (
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdraw"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zsvg"
)

func main() {
	zdebug.IsInTests = true
	for _, d := range []time.Duration{
		time.Second * 10,
		// time.Minute,
		// time.Minute * 3,
		// time.Hour,
		// time.Hour * 10,
		// ztime.Day,
		// ztime.Day * 3,
		// ztime.Day * 20,
		// ztime.Day * 200,
		// ztime.Day * 2000,
	} {
		makeGraph(d)
	}
}

func makeGraph(dur time.Duration) {
	imageOut := false
	ext := "svg"
	if imageOut {
		ext = "png"
	}
	filePath := fmt.Sprintf("unittest-%v.%s", dur, ext)
	info := zdraw.MakeGraphInfo()
	info.GraphColor = zgeo.ColorCyan
	info.End = time.Now()
	info.Start = info.End.Add(-dur)
	info.Axis.ValueRange = zmath.MakeRange(0.0, 400.0)
	info.Axis.LineColor = zgeo.ColorWhite
	info.Axis.TextColor = zgeo.ColorWhite
	info.Axis.DrawAxisLine.Horizontal = true
	info.Axis.DrawAxisLine.Vertical = true

	file, err := os.Create(filePath)
	if zlog.OnError(err) {
		return
	}
	s := zgeo.SizeD(1400, 400)
	var renderCanvas zcanvas.BaseCanvaser
	var svg *zsvg.SVGGenerator
	var canvas *zcanvas.Canvas
	if imageOut {
		canvas = zcanvas.New()
		canvas.SetSize(s)
		renderCanvas = canvas
	} else {
		svg = zsvg.NewGenerator(file, s, "histo", nil)
		renderCanvas = svg
	}
	rect := zgeo.Rect{Size: s}
	renderCanvas.SetColor(zgeo.ColorBlack)
	renderCanvas.FillRect(rect, 0)
	info.PullValuesFunc = func(i *int) (val float64, t time.Time) {
		x := float64(*i * 5)
		if x > s.W {
			*i = -1
			return 0, time.Time{}
		}
		t = zdraw.XToTime(rect, x, info.Start, info.End)
		random := rand.Float64()
		val = info.Axis.ValueRange.T(random)
		*i++
		return val, t
	}
	zdraw.DrawGraph(renderCanvas, &info, zgeo.Rect{Size: s})
	if imageOut {
		img := canvas.GoImage(zgeo.RectNull)
		err = png.Encode(file, img)
	} else {
		svg.End()
	}
	file.Close()
	if zlog.OnError(err, filePath) {
		return
	}
	zlog.Info("OPEN", filePath)
	zprocess.RunCommand("open", 2, filePath)

}
