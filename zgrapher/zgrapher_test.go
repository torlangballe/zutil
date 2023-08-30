package zgrapher

import (
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

func TestGrapher(t *testing.T) {
	router := mux.NewRouter()
	g := NewGrapher(router, 20, "test1", "testdata/graphs")
	job := Job{}
	job.ID = "fast1"
	job.MinutesPerPixel = 1
	job.PixelHeight = 40
	job.WindowHours = 4
	job.AlwaysDrawnPixelY = 38
	job.Draw = func(canvas *zcanvas.Canvas, job *Job, start, end time.Time) {
		zlog.Warn("Draw:", start, end)
		add := time.Minute * time.Duration(job.MinutesPerPixel)
		for m := start; m.Sub(end) < 0; m = m.Add(add) {
			x := job.XForTime(m)
			min := m.Minute()
			canvas.SetColor(zgeo.ColorYellow)
			if min%8 == 4 {
				canvas.SetColor(zgeo.ColorRed)
			}
			canvas.StrokeVertical(float64(x), float64(job.PixelHeight), float64(job.PixelHeight-8), 1, zgeo.PathLineButt)
			zlog.Warn("X:", x, job.PixelHeight, job.PixelHeight-8)
		}
	}
	g.AddJob(job)
	select {}
}
