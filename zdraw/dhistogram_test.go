package zdraw

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmath/zhistogram"
)

func TestDrawHistogram(t *testing.T) {
	fmt.Println("TestDrawHistogram")
	canvas := zcanvas.New()
	s := zgeo.SizeD(320, 240)
	canvas.SetSize(s)

	styling := zstyle.Styling{
		Font:    *zgeo.FontNew("Righteous-Regular", 10, zgeo.FontStyleNormal),
		FGColor: zgeo.ColorWhite,
		BGColor: zgeo.ColorYellow,
		Corner:  4,
		Spacing: 5,
		Margin:  zgeo.RectFromMarginSize(zgeo.SizeD(4, 4)),
	}
	barFunc := func(n string) (string, zgeo.Color) {
		return n + "ms", zgeo.ColorBlack
	}
	// barFunc = nil
	opts := HistDrawOpts{
		MaxClassIndex:      0,
		OutlierBelow:       zbool.False,
		OutlierAbove:       zbool.True,
		Styling:            styling,
		PercentCutoff:      60, // If we know the highest percent any of the classes will have, we can set a cutoff to scale them all up
		CriticalClassValue: 0.3,
		BarNameFunc:        barFunc,
	}
	var h zhistogram.Histogram
	h.Setup(0.1, 0, 20)
	h.Classes = []zhistogram.Class{
		zhistogram.Class{4, ""},
		zhistogram.Class{20, ""},
		zhistogram.Class{7, ""},
		zhistogram.Class{0, ""},
		zhistogram.Class{0, ""},
	}
	h.OutlierAbove = 5
	h.OutlierBelow = 3
	DrawHistogram(&h, canvas, zgeo.Rect{Size: s}, opts)

	img := canvas.GoImage(zgeo.RectNull)
	zimage.GoImageToPNGFile(img, "test.png")
}
