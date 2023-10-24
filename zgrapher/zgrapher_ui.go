//go:build zui

package zgrapher

import (
	"path/filepath"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcustom"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type col struct {
	URL string
	X   float64
}

type GraphView struct {
	zcustom.CustomView
	Job             Job
	ImagePathPrefix string
	MinWidth        float64

	drawn       map[string]*zimage.Image
	grapherName string
}

func NewGraphView(job Job, grapherName string) *GraphView {
	v := &GraphView{}
	v.Init(v, job, grapherName)
	return v
}

func (v *GraphView) Init(view zview.View, job Job, grapherName string) {
	v.Job = job
	v.CustomView.Init(view, "graph-view")
	v.grapherName = grapherName

	now := time.Now()
	v.SetDrawHandler(v.draw)
	v.Job.canvasStartTime = job.calculateWindowStart(now)
	v.drawn = map[string]*zimage.Image{}
	ztimer.RepeatForever(float64(v.Job.SecondsPerPixel), func() {
		v.Expose()
	})
}

func (v *GraphView) CalculatedSize(total zgeo.Size) zgeo.Size {
	size := v.Job.PixelSize()
	zfloat.Maximize(&size.W, v.MinWidth)
	zlog.Info("CALC:", size)
	return size
}

func (v *GraphView) draw(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
	// canvas.SetColor(zgeo.ColorBlue)
	// canvas.FillRect(rect)

	// now := time.Now()
	size := v.Job.PixelSize()
	// t := v.Job.calculateWindowStart(now)
	r := zgeo.Rect{Size: size}
	r.Pos.X = rect.Max().X - size.W
	s := v.Job.canvasStartTime
	first := true
	for {
		name := v.Job.storageNameForTime(s)
		// zlog.Info("GraphViewDraw1:", s, r, name)
		if r.Pos.X+size.W <= 0 {
			break
		}
		s = s.Add(-time.Duration(time.Duration(v.Job.WindowMinutes) * time.Minute))
		col := zgeo.ColorRandom()
		canvas.SetColor(col)
		canvas.FillRect(r)
		surl := filepath.Join(v.ImagePathPrefix, "caches", v.grapherName+CachePostfix, name)
		if first || v.drawn[name] == nil {
			zimage.FromPath(surl, func(img *zimage.Image) {
				zlog.Info("GraphViewDraw path got:", surl, img != nil)
				v.drawn[name] = img
				v.Expose()
			})
		}
		img := v.drawn[name]
		if img != nil {
			canvas.DrawImage(img, false, r, 1, zgeo.Rect{Size: img.Size()})
		}
		first = false
		r.Pos.X -= size.W
	}
}
