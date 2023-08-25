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
	id                 string
	windowHours        int
	height             int
	minutesPerPixel    int
	pixelSize          zgeo.Size
	timer              *ztimer.Repeater
	canvas             *zcanvas.Canvas
	drawnUntil         time.Time
	canvasStartTime    time.Time
	currentStorageName string
	draw               func(canvas *zcanvas.Canvas, id string, start, end time.Time)
}

type Grapher struct {
	jobs  []Job
	cache *zfilecache.Cache
}

func NewGrapher(router *mux.Router, days int, id, folderPath string) *Grapher {
	g := &Grapher{}
	g.cache = zfilecache.Init(router, folderPath, "caches/", id+"Cache/")
	g.cache.DeleteAfter = ztime.Day * time.Duration(days)
	g.cache.ServeEmptyImage = true
	g.cache.DeleteRatio = 0.1
	return g
}

func (j *Job) IDString() string {
	return fmt.Sprint(j.id, "_", j.secondsPerPixel)
}

func (j *Job) PixelWidth() int {
	return j.windowHours * 60 / j.minutesPerPixel
}

func (j *Job) PixelSize() zgeo.Size {
	return zgeo.Size{
		X: float64(j.PixelWidth),
		Y: j.height,
	}
}

func (j *Job) StorageName(t time.Time) string {
	year, month, day := t.Date()
	hour := t.Hour()
	return fmt.Sprintf("%s@%02d-%02d-%02dT%02d.png", j.IDString(), year, int(month), day, hour)
}

func (j *Job) calculateWindowStart(t time.Time) time.Time {
	if j.windowHours <= 24 {
		midnight := ztime.GetStartOfDay(t)
		hour := zmath.RoundToMod(t.Hour(), j.windowHours)
		return midnight.Add(time.Hour * time.Duration(hour))
	}
	windowDays := j.windowHours / 24
	days := ztime.DaysSince2000FromTime(t)
	days = int(zmath.RoundToMod(days, windowDays))
	return ztime.TimeOfDaysSince2000(days, t.Location())
}

func (g *Grapher) AddJob(jobID string, windowHours, height, minutesPerPixel int) {
	var job Job
	if windowHours <= 24 {
		zlog.Assert(24%windowHours == 0, windowHours)
	} else {
		zlog.Assert(windowHours%24 == 0, windowHours)
	}
	if minutesPerPixel > 60 {
		zlog.Assert(minutesPerPixel%60 == 0, minutesPerPixel)
	} else {
		zlog.Assert(60%minutesPerPixel == 0, minutesPerPixel)
	}
	job.id = jobID
	job.windowHours = windowHours
	job.height = height
	job.minutesPerPixel = minutesPerPixel
	now := time.Now()
	job.currentStorageName = job.StorageName(now)
	job.canvasStartTime = job.calculateWindowStart(now)
	if !g.findJobFromImage(&job) {
		job.canvas = zcanvas.New()
		job.canvas.SetSize(job.pixelSize)
	}
	job.timer = ztimer.RepeatForeverNow(float64(minutesPerPixel)/60, func() {
		job.update(g)
	})
}

func (j *Job) clampTimeToPixels(t time.Time) time.Time {
	year, month, day := t.Date()
	hour, min, _ := t.Clock()
	min = zmath.RoundToMod(min, j.minutesPerPixel%60)
	if j.minutesPerPixel > 60 {
		hour = zmath.RoundToMod(hour, j.minutesPerPixel/60)
	}
	return time.Date(year, month, day, hour, min, 0, 0, t.Location())
}

func (j *Job) update(g *Grapher) {
	now := time.Now()
	storageName := j.StorageName(now)
	if storageName != j.currentStorageName {
		j.createNewStoredImage(g, now, storageName)
	}
	end := j.clampTimeToPixels(now)
	j.draw(j.canvas, j.id, j.drawnUntil, end)
	j.saveCanvas(g)
	j.drawnUntil = end
}

func (j *Job) createNewStoredImage(g *Grapher, now time.Time, storageName string) {
	j.canvas.Clear()
	j.canvasStartTime = j.calculateWindowStart(now)
	j.currentStorageName = storageName
}

func (j *Job) saveCanvas(g *Grapher) {
	img := j.canvas.GoImage(zgeo.Rect{})
	data, err := zimage.GoImagePNGData(img)
	if zlog.OnError(err) {
		return
	}
	g.cache.CacheFromData(data, j.currentStorageName)
}

func (g *Grapher) findJobFromImage(job *Job) bool {
	fpath, _ := g.cache.GetPathForName(job.currentStorageName)
	if zfile.Exists(fpath) {
		img, _, err := zimage.GoImageFromFile(fpath)
		if zlog.OnError(err) {
			return false
		}
		zlog.Assert(zimage.GoImageZSize(img) == job.pixelSize)
		job.canvas = zcanvas.CanvasFromGoImage(img)
		return true
	}
	return false
}
