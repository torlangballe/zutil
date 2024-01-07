//go:build server && !js

package zgrapher

import (
	"image"
	"image/color"
	"net/http"
	"path"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type SJob struct {
	Job
	AlwaysDrawnPixelY int // alwaysDrawnPixelY is a pixel if clear on a column, hasn't been drawn yet

	image      *image.NRGBA
	drawnUntil time.Time
}

type Grapher struct {
	GrapherBase
	Draw func(img *image.NRGBA, job *SJob, start, end time.Time, first bool) // Called on the second for SecondsPerPixel. Set Draw to render to the image between start and end. If first is true, can get data for multiple jobs

	jobs    zmap.LockMap[string, *SJob]
	cache   *zfilecache.Cache
	looping bool
	timer   *ztimer.Repeater

	renderingOldParts zmap.LockMap[Job, bool]
}

var EnableLog zlog.Enabler

func Init() {
	zlog.RegisterEnabler("zgrapher.Log", &EnableLog)
}

func NewGrapher(router *mux.Router, deleteDays int, grapherName, folderPath string, secondsPerPixel int) *Grapher {
	g := &Grapher{}
	g.SecondsPerPixel = secondsPerPixel
	folderName := makeCacheFoldername(secondsPerPixel, grapherName)
	g.cache = zfilecache.Init(router, folderPath, "caches/", folderName)
	g.cache.InterceptServeFunc = func(w http.ResponseWriter, req *http.Request, file string) bool {
		return interceptServe(g, w, req, file)
	}
	g.cache.DeleteAfter = ztime.Day * time.Duration(deleteDays)
	// g.cache.ServeEmptyImage = true
	g.cache.DeleteRatio = 0.1
	g.cache.NestInHashFolders = false

	ztimer.StartIn(2, func() { // give it some seconds to get started...
		g.updateAll(time.Now())
	})
	return g
}

func interceptServe(g *Grapher, w http.ResponseWriter, req *http.Request, file string) bool {
	// zlog.Info("zgrapher: interceptServe", req.URL.Path, zfile.Exists(file))
	_, fullName := path.Split(req.URL.Path)
	name := fullName
	if !zstr.HasSuffix(name, ".png", &name) {
		return false
	}
	// zlog.Info("zgrapher: No serve?:", name)
	date := zstr.TailUntilWithRest(name, "@", &name)
	// zlog.Info("zgrapher: No serve0?:", name, date)
	if date == "" {
		return false
	}
	// 2023-10-26T1200 21799614_60_120
	sid := zstr.HeadUntil(name, "_")
	if sid == "" {
		return false
	}
	var job SJob
	if sid == "0" {
		if g.jobs.Count() == 0 {
			zlog.Error(nil, "serving sid='0' zgraph")
			return false
		}
		job = *g.jobs.Index(g.jobs.AnyKey())
		job.ID = "0"
	} else {
		j, got := g.jobs.Get(sid)
		if zlog.ErrorIf(!got, sid, "removed job?") {
			return false
		}
		job = *j
	}
	// zlog.Info("zgrapher: No serve2?:", fullName, job.storageName())
	if job.storageName() == fullName {
		return false // it's current, just render as usual
	}
	// zlog.Info("zgrapher: interceptServe: Name not same as current:", fullName, "!=", job.storageName())
	t, err := time.ParseInLocation("2006-01-02T1504", date, time.Local)
	// zlog.Info("zgrapher: No serve2?:", name, t, err)
	if zlog.OnError(err, date) {
		return false
	}
	mod := zfile.Modified(file)
	end := t.Add(time.Duration(job.WindowMinutes) * time.Minute)
	if !mod.IsZero() && !mod.Before(end) {
		zlog.Info(EnableLog, "zgrapher: Has old rendered non-current part with new modified date:", fullName, end, "mod:", mod)
		return false // it has a file and it's modified after end-time
	}
	// zlog.Info("zgrapher: Should render old requested part:", fullName, date, "end:", end, mod.IsZero())
	g.renderOldPart(job, t)
	return false
}

func (g *Grapher) startRenderLoop(windowMinutes int) {
	s := calculateWindowStart(time.Now(), windowMinutes)
	durSecs := int(ztime.Since(s))
	addSecs := (durSecs/g.SecondsPerPixel + 1) * g.SecondsPerPixel
	next := s.Add(ztime.SecondsDur(float64(addSecs)))
	ztimer.StartAt(next, func() {
		g.updateAll(next)
		g.startRenderLoop(windowMinutes)
	})
	// zlog.Info("RenderStart:", s, durSecs, addSecs, g.SecondsPerPixel, "@", next)
}

func (g *Grapher) findJobFromImagePath(job *SJob) bool {
	return false
	fpath, _ := g.cache.GetPathForName(job.storageName())
	if zfile.Exists(fpath) {
		img, _, err := zimage.GoImageFromFile(fpath)
		if zlog.OnError(err) {
			return false
		}
		zlog.Assert(zimage.GoImageZSize(img) == job.PixelSize(&g.GrapherBase))
		w := job.PixelWidth(&g.GrapherBase)
		for x := 0; x < w; x++ {
			c := img.At(x, job.AlwaysDrawnPixelY)
			_, _, _, a := c.RGBA()
			if a != 0 {
				job.drawnUntil = job.TimeForX(&g.GrapherBase, x+1)
				zlog.Warn("findUntil:", x, job.drawnUntil)
				break
			}
		}
		job.image = zimage.GoImageToNRGBA(img)
		return true
	}
	return false
}

func (g *Grapher) AddJob(job SJob) {
	zlog.Assert(job.ID != "")
	zlog.Assert(job.WindowMinutes != 0)
	zlog.Assert(job.PixelHeight != 0)
	zlog.Assert(g.Draw != nil)
	if job.WindowMinutes <= 60 {
		zlog.Assert(60%job.WindowMinutes == 0, job.WindowMinutes)
	} else if job.WindowMinutes <= 24*60 {
		zlog.Assert(24%(job.WindowMinutes/60) == 0, job.WindowMinutes)
	} else {
		zlog.Assert(job.WindowMinutes/60%24 == 0, job.WindowMinutes)
	}
	if g.SecondsPerPixel > 60 {
		zlog.Assert(g.SecondsPerPixel%60 == 0, g.SecondsPerPixel)
	} else {
		zlog.Assert(60%g.SecondsPerPixel == 0, g.SecondsPerPixel)
	}
	now := time.Now()
	// zlog.Warn("JOB:", job.WindowMinutes)
	job.CanvasStartTime = calculateWindowStart(now, job.WindowMinutes)
	job.drawnUntil = job.CanvasStartTime

	// zlog.Warn("Time0:", job.TimeForX(0))
	// zlog.Warn("TimeW:", job.TimeForX(job.PixelWidth()))
	if !g.findJobFromImagePath(&job) {
		s := job.PixelSize(&g.GrapherBase)
		job.image = image.NewNRGBA(zgeo.Rect{Size: s}.GoRect())
	}
	if job.AlwaysDrawnPixelY == 0 {
		job.AlwaysDrawnPixelY = job.PixelHeight - 1
	}
	if !g.looping {
		g.looping = true
		g.startRenderLoop(job.WindowMinutes)
	}
	g.jobs.Set(job.ID, &job)
}

func (g *Grapher) RemoveJob(jobID string) {
	g.jobs.Remove(jobID)
}

func (g *Grapher) AllIDs() map[string]bool {
	m := map[string]bool{}
	g.jobs.ForEach(func(id string, job *SJob) bool {
		m[id] = true
		return true
	})
	return m
}

func (g *Grapher) HasJob(jobID string) bool {
	return g.jobs.Has(jobID)
}

// func (g *Grapher) FindJob(jobID string) (SJob, bool) {
// 	return g.jobs.Get(jobID)
// }

func clampTimeToPixels(g *Grapher, t time.Time) time.Time {
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	if g.SecondsPerPixel < 60 {
		sec = zmath.RoundToMod(sec, g.SecondsPerPixel%60)
	} else if g.SecondsPerPixel < 3600 {
		min = zmath.RoundToMod(min, (g.SecondsPerPixel/60)%60)
	} else {
		hour = zmath.RoundToMod(hour, g.SecondsPerPixel/3600)
	}
	// zlog.Warn("CLAMP:", hour, min, sec)
	return time.Date(year, month, day, hour, min, sec, 0, t.Location())
}

func (g *Grapher) renderOldPart(job SJob, t time.Time) {
	job.CanvasStartTime = t
	_, has := g.renderingOldParts.GetSet(job.Job, true)
	if has {
		return
	}
	// zlog.Info("renderOldPart:", g.SecondsPerPixel, job.ID, t)
	s := job.PixelSize(&g.GrapherBase)
	img := image.NewNRGBA(zgeo.Rect{Size: s}.GoRect())

	zprocess.RunFuncUntilTimeoutSecs(2, func() {
		g.Draw(img, &job, t, t.Add(time.Duration(job.WindowMinutes)*time.Minute), true)
		job.saveToCacheAtTime(g, img, t)
		g.renderingOldParts.Remove(job.Job) // we remove from rending map, it has a file now.
	})
	// zlog.Info("renderOldPartDone:", g.SecondsPerPixel, job.ID, t, job.storageName())
}

func (g *Grapher) updateAll(now time.Time) {
	first := true
	g.jobs.ForEach(func(id string, job *SJob) bool {
		cstart := calculateWindowStart(now, job.WindowMinutes)
		if cstart != job.CanvasStartTime {
			job.CanvasStartTime = cstart
		}
		onePixBack := -time.Duration(g.SecondsPerPixel) * time.Second
		onePixBack = 0
		g.Draw(job.image, job, job.drawnUntil.Add(onePixBack), now, first)
		first = false
		job.saveToCache(g)
		job.drawnUntil = now
		g.jobs.Set(id, job)
		return true
	})
}

func (j *SJob) saveToCacheAtTime(g *Grapher, img *image.NRGBA, t time.Time) {
	data, err := zimage.GoImagePNGData(img)
	if zlog.OnError(err) {
		return
	}
	name := j.storageNameForTime(t)
	_, err = g.cache.CacheFromData(data, name)
	// zlog.Warn("saveToCacheAtTime", name, err, t)
	zlog.OnError(err)
}

func (j *SJob) saveToCache(g *Grapher) {
	j.saveToCacheAtTime(g, j.image, j.CanvasStartTime)
}

func StrokeAndClearVertInImage(img zimage.SetableImage, x, y1, y2 int, col color.Color) {
	h := int(img.Bounds().Dy())
	clear := zgeo.ColorClear.GoColor()
	for y := 0; y <= h; y++ {
		img.Set(x, y, clear)
	}
	for y := y1; y <= y2; y++ {
		img.Set(x, y, col)
	}
}
