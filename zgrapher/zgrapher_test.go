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
	"github.com/torlangballe/zutil/zrest"
)

func TestGrapher(t *testing.T) {
	router := mux.NewRouter()
	g := NewGrapher(router, 20, "grapherTest", ".testdata")
	job := SJob{}
	job.ID = "test1"
	job.SecondsPerPixel = 5
	job.PixelHeight = 20
	job.WindowMinutes = 20
	job.AlwaysDrawnPixelY = 18
	i := 0
	rainbow := []zgeo.Color{zgeo.ColorRed, zgeo.ColorOrange, zgeo.ColorYellow, zgeo.ColorGreen, zgeo.ColorBlue, zgeo.ColorMagenta, zgeo.ColorPurple}
	job.Draw = func(canvas *zcanvas.Canvas, job *SJob, start, end time.Time) {
		// zlog.Warn("Draw:", start, end)
		add := time.Second * time.Duration(job.SecondsPerPixel)
		for m := start; m.Sub(end) < 0; m = m.Add(add) {
			x := job.XForTime(m)
			secs := m.Second()
			zlog.Warn(i, "X:", x, job.PixelHeight, job.PixelHeight-8, m)
			col := rainbow[x%len(rainbow)]
			canvas.SetColor(col)
			y := (secs / 4) % job.PixelHeight
			canvas.StrokeVertical(float64(x), float64(job.PixelHeight), float64(job.PixelHeight-y), 1, zgeo.PathLineButt)
			i++
		}
	}
	g.AddJob(job)
	zlog.Warn("Serve on :7776")
	zrest.LegalCORSOrigins["http://localhost:7777"] = true // 7777 because it's where origin is from not us
	znet.ServeHTTPInBackground(":7776", "", router)
	select {}
}
