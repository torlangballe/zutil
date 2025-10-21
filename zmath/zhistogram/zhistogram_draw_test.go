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
	// barFunc := func(n string) (string, zgeo.Color) {
	// 	return n + "ms", zgeo.ColorBlack
	// }
	opts := DrawOpts{
		MaxClassIndex:      0,
		OutlierBelow:       zbool.False,
		OutlierAbove:       zbool.True,
		Styling:            styling,
		PercentCutoff:      60, // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
		CriticalClassValue: 3300,
		// BarNameFunc:        barFunc,
	}
	var h Histogram
	// h.Setup(0.1, 0, 0.4)
	// h.Add(0.1)
	// h.Add(0.1)
	// h.Add(0.1)
	// h.Add(0.1)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.2)
	// h.Add(0.3)
	// h.Add(0.3)
	// h.Add(0.35)
	// h.Add(0.6)
	// h.Add(0.6)
	// h.Add(0.6)
	// h.Add(0.6)
	// h.Add(0.6)
	// // h.OutlierAbove = 5

	h.SetupNamedRanges(0, 30, "30s", 60, "1m", 300, "5m", 3600, "1h")
	h.Add(10)
	h.Add(22)
	h.Add(5)
	h.Add(33)
	h.Add(16)
	h.Add(44)
	h.Add(55)
	h.Add(33)
	h.Add(70)
	h.Add(71)
	h.Add(72)
	h.Add(60)
	h.Add(155)
	h.Add(222)
	h.Add(211)
	h.Add(333)
	h.Add(555)
	h.Add(4444)
	file, err := os.Create("unittest.svg")
	if err != nil {
		t.Error(err)
		return
	}
	svg := zsvg.NewGenerator(file, zgeo.SizeD(400, 300), "histo", nil)
	h.Draw(svg, zgeo.Rect{Size: s}, opts)
	svg.End()
	file.Close()
}
