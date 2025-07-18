//go:build zui

package zgrapher

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcustom"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type col struct {
	URL string
	X   float64
}

type GraphView struct {
	GrapherBase
	zcustom.CustomView
	Job                 Job
	ImagePathPrefix     string
	MinWidth            float64
	TickColor           zgeo.Color
	TickYRange          zmath.Range[float64]
	On                  bool
	Ticks               int
	ShowMarkerAt        time.Time
	EndMarkerAt         time.Time
	ShowTicksText       bool
	HandleSimplePressed func(t time.Time)
	HandleSelectedTime  func(from, to time.Time)

	drawn       map[string]*zimage.Image
	grapherName string
	repeater    *ztimer.Repeater
	timer       *ztimer.Timer

	dragStartTime time.Time
	dragEndTime   time.Time
	dragStartX    float64
}

func NewGraphView(id, grapherName string, pixelHeight int) *GraphView {
	v := &GraphView{}
	v.Init(v, id, grapherName, pixelHeight)
	return v
}

// func (v *GraphView) UpdateOn(on bool) {
// 	if v.On != on {
// 		v.On = on
// 		v.requestParts()
// 		v.Expose()
// 	}
// }

func (v *GraphView) Init(view zview.View, id, grapherName string, height int) {
	v.CustomView.Init(view, "graph-view")
	v.grapherName = grapherName
	v.Job.PixelHeight = height
	v.Job.ID = id
	v.TickColor = zgeo.ColorDarkGray
	v.SetStroke(1, zgeo.ColorLightGray, true)
	v.SetDrawHandler(v.draw)
	v.drawn = map[string]*zimage.Image{}
	v.repeater = ztimer.RepeaterNew()
	v.timer = ztimer.TimerNew()
	v.ShowTicksText = true
	v.AddOnRemoveFunc(func() {
		// zlog.Info("GraphView remove", id, grapherName)
		v.repeater.Stop()
		for _, img := range v.drawn {
			img.Release()
		}
	})
	v.SetPressUpDownMovedHandler(v.handleGraphUpDownMoved)
}

func (v *GraphView) Update(secsPerPixel int, windowMinutes int, ticks int, on bool) bool {
	if v.Job.WindowMinutes != windowMinutes && v.SecondsPerPixel != secsPerPixel || v.On != on {
		v.On = on
		v.Job.WindowMinutes = windowMinutes
		v.SecondsPerPixel = secsPerPixel
		v.Ticks = ticks
		v.TickYRange = zmath.MakeRange(0, float64(v.Job.PixelHeight)/7) // 2.5
		if on {
			v.repeater.Set(float64(v.SecondsPerPixel), true, func() bool {
				v.requestParts()
				return true
			})
		} else {
			v.repeater.Stop()
		}
		v.Expose()
		return true
	}
	return false
}

func (v *GraphView) CalculatedSize(total zgeo.Size) (s, max zgeo.Size) {
	s = v.Job.PixelSize(&v.GrapherBase)
	zfloat.Maximize(&s.W, v.MinWidth)
	// zlog.Info("CALC:", v.Hierarchy(), v.Job.WindowMinutes, v.SecondsPerPixel, size)
	return s, zgeo.Size{}
}

func localNow() time.Time {
	return ztime.GetTimeWithServerLocation(time.Now())
}

func (v *GraphView) forEachPart(got func(name string, r zgeo.Rect, first bool)) float64 {
	size := v.Job.PixelSize(&v.GrapherBase)
	rsize := v.LocalRect().Size
	r := zgeo.Rect{Size: size}
	r.Pos.X = v.LocalRect().Max().X - size.W
	now := localNow()
	sMax := calculateWindowStart(now, v.Job.WindowMinutes)
	v.Job.CanvasStartTime = sMax
	first := true
	count := 0
	// var diff float64
	x := float64(v.Job.XForTime(&v.GrapherBase, now))
	offset := x
	r.Pos.X = rsize.W - size.W + (size.W - x)
	// zlog.Info("Offset:", x)
	back := -time.Duration(time.Duration(v.Job.WindowMinutes) * time.Minute)
	for {
		name := v.Job.storageNameForTime(sMax)
		// zlog.Info("ForEach:", x, r.Pos.X, size.W, r.Pos.X+size.W, name, sMax)
		if r.Pos.X+size.W+offset < 0 {
			break
		}
		got(name, r, first)
		r.Pos.X -= size.W
		x -= size.W
		sMax = sMax.Add(back)
		first = false
		count++
	}
	return offset
}

func (v *GraphView) requestParts() {
	if !v.On || v.Job.WindowMinutes == 0 {
		return
	}
	v.forEachPart(func(name string, r zgeo.Rect, first bool) {
		if !first && v.drawn[name] != nil {
			return
		}
		folderName := makeCacheFoldername(v.SecondsPerPixel, v.grapherName)
		surl := zfile.JoinPathParts(v.ImagePathPrefix, "zgrapher", folderName, name)
		surl += "?tick=" + zstr.GenerateRandomHexBytes(12)
		zimage.FromPath(surl, false, func(img *zimage.Image) {
			if img == nil {
				// zlog.Info("No image request parts:", surl, v.On)
				// if !first {
				v.timer.StartIn(5, func() {
					v.requestParts()
				})
				// }
				return
			}
			// zlog.Info("Got image request part:", surl)
			old := v.drawn[name]
			if old != nil {
				// zlog.Info("GraphView remove old image", v.grapherName, name, zlog.Pointer(old), zlog.Pointer(img))
				old.Release()
			}
			v.drawn[name] = img // set img before expose, expose is immediate draw on web at least
			v.Expose()
			// zlog.Info("GraphView Draw got image:", surl)
		})
	})
}

func (v *GraphView) draw(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
	if !v.On || v.Job.WindowMinutes == 0 {
		return
	}
	r := rect
	r.Size.W -= float64(v.Job.PixelWidth(&v.GrapherBase))
	canvas.SetColor(zgeo.ColorNewGray(0, 0.05))
	canvas.FillRect(r)

	// zlog.Info("draw", v.SecondsPerPixel, v.Job.ID, rect)
	i := 0
	offset := v.forEachPart(func(name string, r zgeo.Rect, first bool) {
		img := v.drawn[name]
		// zlog.Info("GraphView Draw:", img != nil, name, r)
		// canvas.SetColor(zgeo.ColorRandom())
		// canvas.FillRect(r)
		if img != nil {
			canvas.DrawImage(img, false, r, 1, zgeo.Rect{Size: img.Size()})
		}
		i++
	})
	if !v.dragStartTime.IsZero() && !v.dragEndTime.IsZero() {
		xs := v.xForTime(v.dragStartTime)
		xe := v.xForTime(v.dragEndTime)
		r := rect.ExpandedD(-1)
		r.SetMinX(xs)
		r.SetMaxX(xe)
		canvas.SetColor(zgeo.ColorNew(1, 1, 0, 0.5))
		path := zgeo.PathNewRect(r, zgeo.SizeBoth(2))
		canvas.DrawPath(path, zgeo.ColorBlack, 3, zgeo.PathLineRound, true)
	}
	if !v.ShowMarkerAt.IsZero() {
		x := v.xForTime(v.ShowMarkerAt)
		drawMarker(float64(x), rect, canvas, zgeo.ColorRed)
		if !v.EndMarkerAt.IsZero() {
			x2 := v.xForTime(v.ShowMarkerAt)
			drawMarker(float64(x2), rect, canvas, zgeo.ColorDarkGreen)
		}
	}
	v.drawHours(canvas, offset)
}

func drawMarker(x float64, rect zgeo.Rect, canvas *zcanvas.Canvas, color zgeo.Color) {
	canvas.SetColor(zgeo.ColorBlack)
	y := rect.Max().Y
	canvas.StrokeVertical(x, y-3, y, 3, zgeo.PathLineButt)
	canvas.SetColor(color)
	canvas.StrokeVertical(x, y-6, y-2, 1, zgeo.PathLineButt)
}

func (v *GraphView) xForTime(t time.Time) float64 {
	t = t.UTC()
	w := v.LocalRect().Size.W
	d := time.Since(t) / time.Second / time.Duration(v.SecondsPerPixel)
	return w - float64(d)
}

func (v *GraphView) TimeForX(x int) time.Time {
	w := int(v.LocalRect().Size.W)
	d := time.Duration((w-x)*v.SecondsPerPixel) * time.Second
	return localNow().Add(-d).UTC() // why utc???
}

func getTimeInt(t time.Time, span time.Duration) (n, n2 int, important bool) {
	// zlog.Info("getTimeInt span:", span, t)
	if span <= time.Hour*3 {
		n = t.Minute()
		return n, t.Hour(), n%60 == 0
	}
	if span <= ztime.Day {
		n = t.Hour()
		return n, -1, true
	}
	n = t.Day()
	return n, -1, true
}

func (v *GraphView) drawHours(canvas *zcanvas.Canvas, xOffset float64) {
	if !v.TickColor.Valid {
		return
	}
	canvas.SetColor(v.TickColor)
	end := localNow()
	w := int(v.LocalRect().Size.W)
	pixWidth := v.Job.PixelWidth(&v.GrapherBase)
	span := time.Duration(w*v.SecondsPerPixel) * time.Second
	shortSpan := time.Duration(pixWidth*v.SecondsPerPixel) * time.Second
	start := end.Add(-span)
	ticks := v.Ticks * (w / pixWidth)
	if ticks == 0 {
		return
	}
	// zlog.Info("BeforeNice:", start, end, int(v.Rect().Size.W/3))
	inc, _, begin := ztime.NiceAxisIncrements(start, end, int(v.Rect().Size.W/3))
	// zlog.Warn("drawHours1:", v.Job.ID, start, end, span, inc, ticks, v.SecondsPerPixel)
	// zlog.Warn("drawHours1:", v.Job.ID, ticks, v.Ticks, w, v.Job.PixelWidth(&v.GrapherBase), begin, end)
	for t := begin; t.Before(end); t = t.Add(inc) {
		// zlog.Warn("drawHours:", t)
		x := float64(v.xForTime(t))
		n, n2, doText := getTimeInt(t, shortSpan)
		canvas.SetColor(v.TickColor.WithOpacity(0.5))
		y2 := v.TickYRange.Max
		var text2 string
		if v.ShowTicksText {
			if t.Hour() == 0 && t.Minute() == 0 && t.Second() == 0 {
				if inc < ztime.Day {
					y2 = v.LocalRect().Size.H
					text2 = t.Weekday().String()[:3]
				} else if t.Day() == 1 {
					y2 = v.LocalRect().Size.H
					text2 = t.Month().String()[:3]
				}
			}
		}
		canvas.StrokeVertical(x, v.TickYRange.Min, y2, 1, zgeo.PathLineButt)

		if doText {
			ti := ztextinfo.New()
			ti.Alignment = zgeo.TopLeft
			ti.Color = v.TickColor
			ti.Text = fmt.Sprintf("%02d", n)
			if n2 != -1 {
				ti.Text = strconv.Itoa(n2) + ":" + ti.Text
			}
			ti.Font = zgeo.FontNice(8, zgeo.FontStyleNormal)
			ti.Rect = zgeo.RectFromXYWH(x+1, v.TickYRange.Min+2, 20, 10)
			ti.Rect.Pos.Y = ti.Draw(canvas).Max().Y
			if text2 != "" {
				ti.Text = strings.ToLower(text2)
				ti.Color = v.TickColor.WithOpacity(0.7)
				ti.Draw(canvas)
			}
		}
	}
}

func (v *GraphView) ClearSelectedTime() {
	zlog.Info("GV.ClearSelectedTime")
	v.dragStartTime = time.Time{}
	v.dragEndTime = time.Time{}
	v.Expose()
}

func (v *GraphView) handleGraphUpDownMoved(pos zgeo.Pos, down zbool.BoolInd) bool {
	switch down {
	case zbool.True:
		if zkeyboard.ModifiersAtPress != 0 {
			return true
		}
		if !v.dragStartTime.IsZero() {
			v.ClearSelectedTime()
			if v.HandleSelectedTime != nil {
				v.HandleSelectedTime(time.Time{}, time.Time{})
			}
			return false
		}
		v.dragStartX = pos.X
		v.dragStartTime = v.TimeForX(int(pos.X))
		return true
	case zbool.Unknown:
		if zkeyboard.ModifiersAtPress != 0 {
			return false
		}
		if !v.dragStartTime.IsZero() && math.Abs(pos.X-v.dragStartX) > 3 {
			v.dragEndTime = v.TimeForX(int(pos.X))
			// zlog.Info("DRAGGING in Select!", v.dragStartTime, v.dragStartTime.IsZero(), v.dragEndTime)
			v.Expose()
		}
		return true
	case zbool.False:
		if v.dragEndTime.IsZero() {
			if v.HandleSimplePressed != nil {
				v.HandleSimplePressed(v.dragStartTime)
			}
			v.dragStartTime = time.Time{}
			return true
		}
		v.dragEndTime = v.TimeForX(int(pos.X))
		if v.dragEndTime.Before(v.dragStartTime) {
			zreflect.Swap(&v.dragStartTime, &v.dragEndTime)
		}
		if v.HandleSelectedTime != nil {
			v.HandleSelectedTime(v.dragStartTime, v.dragEndTime)
		}
		v.Expose()
		return true
	}
	return true
}
