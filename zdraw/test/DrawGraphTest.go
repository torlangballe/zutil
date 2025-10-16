package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zdraw"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zprocess"
)

func main() {
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
	info := zdraw.MakeGraphInfo()
	info.GraphColor = zgeo.ColorCyan
	info.End = time.Now()
	info.Start = info.End.Add(-dur)
	info.Axis.ValueRange = zmath.MakeRange(0.0, 300.0)
	s := zgeo.SizeD(600, 400)
	canvas := zcanvas.New()
	canvas.SetSize(s)
	rect := zgeo.Rect{Size: s}
	canvas.SetColor(zgeo.ColorBlack)
	canvas.FillRect(rect, 0)
	info.PullValuesFunc = func(i *int) (val float64, t time.Time) {
		if float64(*i) > s.W {
			*i = -1
			return 0, time.Time{}
		}
		t = zdraw.XToTime(rect, float64(*i), info.Start, info.End)
		random := rand.Float64()
		val = info.Axis.ValueRange.T(random)
		*i++
		return val, t
	}
	zdraw.DrawGraph(&info, zgeo.Rect{Size: s}, canvas)
	img := canvas.GoImage(zgeo.RectNull)
	file := fmt.Sprintf("unittest-%v.png", dur)
	// zlog.Warn("FILE:", file)
	zimage.GoImageToPNGFile(img, file)
	zprocess.RunCommand("open", 2, file)

}
