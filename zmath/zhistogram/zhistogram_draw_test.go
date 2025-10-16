package zhistogram

import (
	"os"
	"testing"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zsvg"
)

func TestDrawHistogram(t *testing.T) {
	zlog.Warn("TestDrawHistogram")
	canvas := zcanvas.New()
	s := zgeo.SizeD(320, 240)
	canvas.SetSize(s)

	styling := MakeDefaultStyling(s)
	barFunc := func(n string) (string, zgeo.Color) {
		return n + "ms", zgeo.ColorBlack
	}
	opts := DrawOpts{
		MaxClassIndex:      0,
		OutlierBelow:       zbool.False,
		OutlierAbove:       zbool.True,
		Styling:            styling,
		PercentCutoff:      60, // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
		CriticalClassValue: 0.3,
		BarNameFunc:        barFunc,
	}
	var h Histogram
	h.Setup(0.1, 0, 0.4)
	h.Add(0.1)
	h.Add(0.1)
	h.Add(0.1)
	h.Add(0.1)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.2)
	h.Add(0.3)
	h.Add(0.3)
	h.Add(0.35)
	h.Add(0.6)
	h.Add(0.6)
	h.Add(0.6)
	h.Add(0.6)
	h.Add(0.6)
	// h.OutlierAbove = 5
	file, err := os.Create("test.svg")
	if err != nil {
		t.Error(err)
		return
	}
	svg := zsvg.NewGenerator(file, zgeo.SizeD(400, 300), "histo", 0)
	h.Draw(svg, zgeo.Rect{Size: s}, opts)
	svg.End()
	file.Close()
}
