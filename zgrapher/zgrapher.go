// zgrapher is a package for rendering and displaying horizontal bar-graphs.
//
// They are rendered into png's on the backend, and stored/served using zfilecache.
// They are displayed using GraphView on frontend.

package zgrapher

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/ztime"
)

type Job struct {
	ID              string
	WindowMinutes   int
	PixelHeight     int
	CanvasStartTime time.Time
}

type GrapherBase struct {
	SecondsPerPixel int
}

const CachePostfix = "ZGrapherCache"

func (j *Job) PixelWidth(base *GrapherBase) int {
	if base.SecondsPerPixel == 0 {
		return 0
	}
	return j.WindowMinutes * 60 / base.SecondsPerPixel
}

func (j *Job) PixelSize(base *GrapherBase) zgeo.Size {
	return zgeo.SizeD(float64(j.PixelWidth(base)), float64(j.PixelHeight))
}

func (j *Job) XForTime(base *GrapherBase, t time.Time) int {
	x := int(t.UTC().Sub(j.CanvasStartTime) / time.Second / time.Duration(base.SecondsPerPixel))
	return x
}

func (j *Job) TimeForX(base *GrapherBase, x int) time.Time {
	t := j.CanvasStartTime.Add(time.Duration(base.SecondsPerPixel*x) * time.Second)
	return t
}

func (j *Job) storageName() string {
	return j.storageNameForTime(j.CanvasStartTime)
}

func (j *Job) storageNameForTime(t time.Time) string {
	year, month, day := t.UTC().Date()
	hour := t.Hour()
	min := t.Minute()
	return fmt.Sprintf("%s_%d@%02d-%02d-%02dT%02d%02d.png", j.ID, j.WindowMinutes, year, int(month), day, hour, min)
}

func makeCacheFoldername(secondsPerPixel int, grapherName string) string {
	return fmt.Sprintf("%s-%d-%s", CachePostfix, secondsPerPixel, grapherName)
}

func calculateWindowStart(t time.Time, windowMinutes int) time.Time {
	t = t.UTC()
	if windowMinutes <= 24*60 {
		midnight := ztime.GetStartOfDay(t)
		div := windowMinutes / 60
		hour := t.Hour()
		var min int
		if div != 0 {
			hour = zmath.RoundToMod(hour, div)
		}
		out := midnight.Add(time.Hour * time.Duration(hour))
		if windowMinutes < 60 {
			min = zmath.RoundToMod(t.Minute(), windowMinutes)
			out = out.Add(time.Minute * time.Duration(min))
		}
		return out
	}
	windowDays := windowMinutes / 24 / 60
	days := ztime.DaysSince2000FromTime(t)
	days = int(zmath.RoundToMod(days, windowDays))
	return ztime.TimeOfDaysSince2000(days, t.Location())
}
