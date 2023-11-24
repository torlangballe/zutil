//go:build server

package zgrapher

import (
	"image"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zstr"
)

const (
	windowMinutes   = 5
	secondsPerPixel = 1
	grapherName     = "grapherTest"
	port            = 7776
	imagePathPrefix = "http://localhost:7776/"
)

func TestGrapher(t *testing.T) {
	router := mux.NewRouter()
	storeFolder := ".testdata"
	g := NewGrapher(router, 20, grapherName, storeFolder, secondsPerPixel)
	zfile.RemoveContents(storeFolder)
	job := SJob{}
	job.ID = "test1"
	job.PixelHeight = 20
	job.WindowMinutes = windowMinutes
	job.AlwaysDrawnPixelY = 18
	i := 0
	rainbow := []zgeo.Color{zgeo.ColorRed, zgeo.ColorOrange, zgeo.ColorYellow, zgeo.ColorGreen, zgeo.ColorBlue, zgeo.ColorMagenta, zgeo.ColorPurple}
	g.Draw = func(img *image.NRGBA, job *SJob, start, end time.Time, first bool) {
		// x1 := job.XForTime(&g.GrapherBase, start)
		// x2 := job.XForTime(&g.GrapherBase, end)
		// zlog.Warn("Draw:", start, end, x1, x2)
		add := time.Second * time.Duration(g.SecondsPerPixel)
		for m := start; m.Sub(end) < 0; m = m.Add(add) {
			x := job.XForTime(&g.GrapherBase, m)
			if i == -1 {
				zlog.Warn(i, "X negative!:", start, x, job.PixelHeight, job.PixelHeight-8, m)
			}
			col := rainbow[x%len(rainbow)]
			StrokeAndClearVertInImage(img, x, 0, job.PixelHeight, col)
			i++
		}
	}
	g.AddJob(job)
	znet.ServeHTTPInBackground(":7776", "", router)
	time.Sleep(time.Second * 1)
	now := time.Now()
	if !checkImageForTime(t, now, job, *g) {
		return
	}
	before := now.Add(-time.Minute * time.Duration(windowMinutes+2))
	if !checkImageForTime(t, before, job, *g) {
		return
	}
}

func checkImageForTime(t *testing.T, at time.Time, job SJob, g Grapher) bool {
	start := calculateWindowStart(at, windowMinutes)
	name := job.storageNameForTime(start)
	folderName := makeCacheFoldername(g.SecondsPerPixel, grapherName)
	surl := zfile.JoinPathParts(imagePathPrefix, "caches", folderName, name)
	surl += "?tick=" + zstr.GenerateRandomHexBytes(12)
	// zlog.Info("Request:", r, surl)
	img, _, err := zimage.GoImageFromURL(surl)
	if err != nil || img == nil {
		t.Error("get image", err)
		return false
	}
	var found bool
	for x := 0; x < img.Bounds().Dx(); x++ {
		c := img.At(x, job.PixelHeight/2)
		_, _, _, a := c.RGBA()
		if a != 0 {
			found = true
		}
	}
	if !found {
		t.Error("No pixels found in rendered graph:", surl)
		return false
	}
	return true
}
