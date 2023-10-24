//go:build server

package zgrapher

import (
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
)

func TestGrapher(t *testing.T) {
	router := mux.NewRouter()
	g := NewGrapher(router, 20, "grapherTest", ".testdata")
	job := JobServer{}
	job.ID = "test1"
	job.SecondsPerPixel = 5
	job.PixelHeight = 40
	job.WindowMinutes = 10
	job.AlwaysDrawnPixelY = 38
	i := 0
	job.Draw = func(canvas *zcanvas.Canvas, job *JobServer, start, end time.Time) {
		zlog.Warn("Draw:", start, end)
		add := time.Second * time.Duration(job.SecondsPerPixel)
		for m := start; m.Sub(end) < 0; m = m.Add(add) {
			x := job.XForTime(m)
			min := m.Minute()
			canvas.SetColor(zgeo.ColorYellow)
			if min%8 == 4 {
				canvas.SetColor(zgeo.ColorRed)
			}
			secs := m.Second()
			y := secs % job.PixelHeight
			canvas.StrokeVertical(float64(x), float64(job.PixelHeight), float64(job.PixelHeight-y), 1, zgeo.PathLineButt)
			zlog.Warn(i, "X:", x, job.PixelHeight, job.PixelHeight-8, m)
			i++
		}
	}
	g.AddJob(job)
	znet.ServeHTTPInBackground(":7776", "", router)
	select {}
}
