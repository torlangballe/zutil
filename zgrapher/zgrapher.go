package zgrapher

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/ztime"
)

type Job struct {
	ID              string
	WindowMinutes   int
	PixelHeight     int
	SecondsPerPixel int
	canvasStartTime time.Time
}

const CachePostfix = "ZGrapherCache/"

func (j *Job) calculateWindowStart(t time.Time) time.Time {
	zlog.Warn("Calc2", j.WindowMinutes)
	if j.WindowMinutes <= 24*60 {

		midnight := ztime.GetStartOfDay(t)
		div := j.WindowMinutes / 60
		hour := t.Hour()
		var min int
		if div != 0 {
			hour = zmath.RoundToMod(hour, div)
		}
		out := midnight.Add(time.Hour * time.Duration(hour))
		if j.WindowMinutes < 60 {
			zlog.Warn("ROUND", j.WindowMinutes)
			min = zmath.RoundToMod(t.Minute(), j.WindowMinutes)
			out = out.Add(time.Minute * time.Duration(min))
		}
		zlog.Warn("calc:", hour, min)
		return out
	}
	windowDays := j.WindowMinutes / 24 / 60
	days := ztime.DaysSince2000FromTime(t)
	days = int(zmath.RoundToMod(days, windowDays))
	return ztime.TimeOfDaysSince2000(days, t.Location())
}

func (j *Job) IDString() string {
	return fmt.Sprint(j.ID, "_", j.SecondsPerPixel)
}

func (j *Job) PixelWidth() int {
	return j.WindowMinutes * 60 / j.SecondsPerPixel
}

func (j *Job) PixelSize() zgeo.Size {
	return zgeo.Size{
		W: float64(j.PixelWidth()),
		H: float64(j.PixelHeight),
	}
}

func (j *Job) XForTime(t time.Time) int {
	x := int(t.Sub(j.canvasStartTime) / time.Second / time.Duration(j.SecondsPerPixel))
	return j.PixelWidth() - x - 1
}

func (j *Job) storageName() string {
	return j.storageNameForTime(j.canvasStartTime)
}

func (j *Job) storageNameForTime(t time.Time) string {
	year, month, day := t.Date()
	hour := t.Hour()
	min := t.Minute()
	return fmt.Sprintf("%s@%02d-%02d-%02dT%02d%02d.png", j.IDString(), year, int(month), day, hour, min)
}
