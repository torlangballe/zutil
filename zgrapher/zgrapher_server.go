package zgrapher

import (
	"fmt"
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

type Job struct {
	ID                string
	WindowHours       int
	PixelHeight       int
	MinutesPerPixel   int
	Draw              func(canvas *zcanvas.Canvas, jon *Job, start, end time.Time)
	AlwaysDrawnPixelY int // alwaysDrawnPixelY is a pixel if clear on a column, hasn't been drawn yet

	timer           *ztimer.Repeater
	canvas          *zcanvas.Canvas
	drawnUntil      time.Time
	canvasStartTime time.Time
}

type Grapher struct {
	jobs  []Job
	cache *zfilecache.Cache
}

func NewGrapher(router *mux.Router, deleteDays int, id, folderPath string) *Grapher {
	g := &Grapher{}
	g.cache = zfilecache.Init(router, folderPath, "caches/", id+"Cache/")
	g.cache.DeleteAfter = ztime.Day * time.Duration(deleteDays)
	g.cache.ServeEmptyImage = true
	g.cache.DeleteRatio = 0.1
	g.cache.NestInHashFolders = false
	return g
}

func (g *Grapher) findJobFromImage(job *Job) bool {
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

func (g *Grapher) AddJob(job Job) *Job {
	zlog.Assert(job.ID != "")
	zlog.Assert(job.WindowHours != 0)
	zlog.Assert(job.PixelHeight != 0)
	zlog.Assert(job.Draw != nil)
	if job.WindowHours <= 24 {
		zlog.Assert(24%job.WindowHours == 0, job.WindowHours)
	} else {
		zlog.Assert(job.WindowHours%24 == 0, job.WindowHours)
	}
	if job.MinutesPerPixel > 60 {
		zlog.Assert(job.MinutesPerPixel%60 == 0, job.MinutesPerPixel)
	} else {
		zlog.Assert(60%job.MinutesPerPixel == 0, job.MinutesPerPixel)
	}
	now := time.Now()
	job.canvasStartTime = job.calculateWindowStart(now)
	job.drawnUntil = job.canvasStartTime

	// zlog.Warn("Time0:", job.TimeForX(0))
	// zlog.Warn("TimeW:", job.TimeForX(job.PixelWidth()))
	if !g.findJobFromImage(&job) {
		job.canvas = zcanvas.New()
		job.canvas.SetSize(job.PixelSize())
	}
	job.timer = ztimer.RepeatForeverNow(float64(job.MinutesPerPixel)*60, func() {
		zlog.Warn("Repeat", job.canvasStartTime)
		job.update(g)
	})
	g.jobs = append(g.jobs, job)
	j := &g.jobs[len(g.jobs)-1]
	return j
}

func (j *Job) IDString() string {
	return fmt.Sprint(j.ID, "_", j.MinutesPerPixel)
}

func (j *Job) PixelWidth() int {
	return j.WindowHours * 60 / j.MinutesPerPixel
}

func (j *Job) PixelSize() zgeo.Size {
	return zgeo.Size{
		W: float64(j.PixelWidth()),
		H: float64(j.PixelHeight),
	}
}

func (j *Job) storageName() string {
	year, month, day := j.canvasStartTime.Date()
	hour := j.canvasStartTime.Hour()
	return fmt.Sprintf("%s@%02d-%02d-%02dT%02d.png", j.IDString(), year, int(month), day, hour)
}

func (j *Job) clampTimeToPixels(t time.Time) time.Time {
	year, month, day := t.Date()
	hour, min, _ := t.Clock()
	min = zmath.RoundToMod(min, j.MinutesPerPixel%60)
	if j.MinutesPerPixel > 60 {
		hour = zmath.RoundToMod(hour, j.MinutesPerPixel/60)
	}
	return time.Date(year, month, day, hour, min, 0, 0, t.Location())
}

func (j *Job) calculateWindowStart(t time.Time) time.Time {
	if j.WindowHours <= 24 {
		midnight := ztime.GetStartOfDay(t)
		hour := zmath.RoundToMod(t.Hour(), j.WindowHours)
		return midnight.Add(time.Hour * time.Duration(hour))
	}
	windowDays := j.WindowHours / 24
	days := ztime.DaysSince2000FromTime(t)
	days = int(zmath.RoundToMod(days, windowDays))
	return ztime.TimeOfDaysSince2000(days, t.Location())
}

func (j *Job) update(g *Grapher) {
	now := time.Now()
	cstart := j.calculateWindowStart(now)
	if cstart != j.canvasStartTime {
		zlog.Info("NewCanvas", cstart)
		j.canvasStartTime = cstart
		j.canvas.Clear()
	}
	end := j.clampTimeToPixels(now)
	j.Draw(j.canvas, j, j.drawnUntil, end)
	j.saveCanvas(g)
	j.drawnUntil = end
}

func (j *Job) saveCanvas(g *Grapher) {
	img := j.canvas.GoImage(zgeo.Rect{})
	data, err := zimage.GoImagePNGData(img)
	if zlog.OnError(err) {
		return
	}
	_, err = g.cache.CacheFromData(data, j.storageName())
	zlog.OnError(err)
	zlog.Warn("saveCanvas")
}

func (j *Job) XForTime(t time.Time) int {
	x := int(t.Sub(j.canvasStartTime) / time.Minute / time.Duration(j.MinutesPerPixel))
	return j.PixelWidth() - x - 1
}

func (j *Job) TimeForX(x int) time.Time {
	x = j.PixelWidth() - x
	t := j.canvasStartTime.Add(time.Duration(j.MinutesPerPixel*x) * time.Minute)
	return t
}

func (job *Job) DrawHours(canvas *zcanvas.Canvas, y, height int, black bool, at time.Time) {

}
