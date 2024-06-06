package zscreen

import (
	"runtime"

	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

//  Created by Tor Langballe on /12/11/15.

type ScreenLayout int

const (
	ScreenPortrait ScreenLayout = iota
	ScreenPortraitUpsideDown
	ScreenLandscapeLeft
	ScreenLandscapeRight
)

type Screen struct {
	IsMain     bool
	isLocked   bool
	ID         string
	Rect       zgeo.Rect
	UsableRect zgeo.Rect
	Scale      float64 //= float64(UIScreen.main.scale)
	SoftScale  float64 // = 1.0
	//	KeyboardRect *zgeo.Rect
}

var (
	mainSoftScale float64
	mainScale     float64
)

func init() {
	zimage.MainScreenScaleFunc = MainScale
}

func GetMain() Screen {
	// test:
	// zlog.Info("ScreenMain:", zlog.GetCallingStackString())
	for _, s := range GetAll() {
		// zlog.Info("Screen:", s.ID, s.Rect, s.IsMain)
		if s.IsMain {
			return s
		}
	}
	if runtime.GOOS != "linux" {
		zlog.Error("No screen!", zlog.CallingStackString())
	}
	s := Screen{}
	s.SoftScale = 1
	s.Scale = 1
	s.IsMain = true
	return s
}

func MainSoftScale() float64 {
	if mainSoftScale != 0 {
		return mainSoftScale
	}
	mainSoftScale = GetMain().SoftScale
	return mainSoftScale
}

func MainScale() float64 {
	if mainScale != 0 {
		return mainScale
	}
	mainScale = GetMain().Scale
	return mainScale
}

func FindForID(id string) *Screen {
	for _, s := range GetAll() {
		if s.ID == id {
			return &s
		}
	}
	return nil
}

func normalizedRect(x, y, w, h float64) zgeo.Rect {
	r := zgeo.RectFromXYWH(x, -y, w, -h)
	return r.NormalizedNegativeSize()
}

func (s *Screen) ScaledSize() zgeo.Size {
	return s.Rect.Size.TimesD(s.Scale)
}
