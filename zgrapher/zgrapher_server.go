//go:build server && !js

package zgrapher

import (
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type JobServer struct {
	Job
	Draw              func(canvas *zcanvas.Canvas, job *JobServer, start, end time.Time)
	AlwaysDrawnPixelY int // alwaysDrawnPixelY is a pixel if clear on a column, hasn't been drawn yet

	timer      *ztimer.Repeater
	canvas     *zcanvas.Canvas
	drawnUntil time.Time
}

type Grapher struct {
	jobs  []JobServer
	cache *zfilecache.Cache
}

func NewGrapher(router *mux.Router, deleteDays int, grapherName, folderPath string) *Grapher {
	g := &Grapher{}
	g.cache = zfilecache.Init(router, folderPath, "caches/", grapherName+CachePostfix)
	g.cache.DeleteAfter = ztime.Day * time.Duration(deleteDays)
	g.cache.ServeEmptyImage = true
	g.cache.DeleteRatio = 0.1
	g.cache.NestInHashFolders = false
	return g
}

func (g *Grapher) findJobFromImage(job *JobServer) bool {
	fpath, _ := g.cache.GetPathForName(job.storageName())
	if zfile.Exists(fpath) {
		img, _, err := zimage.GoImageFromFile(fpath)
		if zlog.OnError(err) {
			return false
		}
		zlog.Assert(zimage.GoImageZSize(img) == job.PixelSize())
		w := job.PixelWidth()
		for x := 0; x < w; x++ {
			c := img.At(x, job.AlwaysDrawnPixelY)
			_, _, _, a := c.RGBA()
			if a != 0 {
				job.drawnUntil = job.TimeForX(x + 1)
				zlog.Warn("findUntil:", x, job.drawnUntil)
				break
			}
		}
		job.canvas = zcanvas.CanvasFromGoImage(img)
		return true
	}
	return false
}

func (g *Grapher) AddJob(job JobServer) *JobServer {
	zlog.Assert(job.ID != "")
	zlog.Assert(job.WindowMinutes != 0)
	zlog.Assert(job.PixelHeight != 0)
	zlog.Assert(job.Draw != nil)
	if job.WindowMinutes <= 60 {
		zlog.Assert(60%job.WindowMinutes == 0, job.WindowMinutes)
	} else if job.WindowMinutes <= 24*60 {
		zlog.Assert(24%(job.WindowMinutes/60) == 0, job.WindowMinutes)
	} else {
		zlog.Assert(job.WindowMinutes/60%24 == 0, job.WindowMinutes)
	}
	if job.SecondsPerPixel > 60 {
		zlog.Assert(job.SecondsPerPixel%60 == 0, job.SecondsPerPixel)
	} else {
		zlog.Assert(60%job.SecondsPerPixel == 0, job.SecondsPerPixel)
	}
	now := time.Now()
	// zlog.Warn("JOB:", job.WindowMinutes)
	job.canvasStartTime = job.calculateWindowStart(now)
	job.drawnUntil = job.canvasStartTime

	// zlog.Warn("Time0:", job.TimeForX(0))
	// zlog.Warn("TimeW:", job.TimeForX(job.PixelWidth()))
	if !g.findJobFromImage(&job) {
		job.canvas = zcanvas.New()
		job.canvas.SetSize(job.PixelSize())
	}
	job.timer = ztimer.RepeatForeverNow(float64(job.SecondsPerPixel), func() {
		zlog.Warn("Repeat", job.canvasStartTime)
		job.update(g)
	})
	g.jobs = append(g.jobs, job)
	j := &g.jobs[len(g.jobs)-1]
	return j
}

func (j *JobServer) clampTimeToPixels(t time.Time) time.Time {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	if j.SecondsPerPixel < 60 {
		sec = zmath.RoundToMod(sec, j.SecondsPerPixel%60)
	} else if j.SecondsPerPixel < 3600 {
		min = zmath.RoundToMod(min, (j.SecondsPerPixel/60)%60)
	} else {
		hour = zmath.RoundToMod(hour, j.SecondsPerPixel/3600)
	}
	// zlog.Warn("CLAMP:", hour, min, sec)
	return time.Date(year, month, day, hour, min, sec, 0, t.Location())
}

func (j *JobServer) update(g *Grapher) {
	now := time.Now()
	cstart := j.calculateWindowStart(now)
	if cstart != j.canvasStartTime {
		zlog.Info("update", cstart)
		j.canvasStartTime = cstart
		j.canvas.Clear()
	}
	end := j.clampTimeToPixels(now)
	j.Draw(j.canvas, j, j.drawnUntil, end)
	j.saveCanvas(g)
	j.drawnUntil = end
}

func (j *JobServer) saveCanvas(g *Grapher) {
	img := j.canvas.GoImage(zgeo.Rect{})
	data, err := zimage.GoImagePNGData(img)
	if zlog.OnError(err) {
		return
	}
	name := j.storageName()
	_, err = g.cache.CacheFromData(data, name)
	zlog.OnError(err)
	zlog.Warn("saveCanvas", img.Bounds().Size(), j.canvas.Size(), g.cache.GetURLForName(name))
}

func (j *JobServer) TimeForX(x int) time.Time {
	x = j.PixelWidth() - x
	t := j.canvasStartTime.Add(time.Duration(j.SecondsPerPixel*x) * time.Second)
	return t
}

func (job *JobServer) DrawHours(canvas *zcanvas.Canvas, y, height int, black bool, at time.Time) {

}
