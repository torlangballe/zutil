//go:build !js

package zscreen

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// #include <CoreGraphics/CoreGraphics.h>
// typedef struct Info {
//     CGRect frame, visibleFrame;
//     int scale;
//     int ismain;
//     long sid;
// } ScreenInfo;
// int GetAll(struct Info *sis, int max);
// void SetMainResolutionWithinWidths(long minw, long minh, long maxw, long maxh);
import "C"

import (
	"strconv"
	"unsafe"

	"github.com/torlangballe/zutil/zgeo"
)

func GetAll() (screens []Screen) {
	var count C.uint32_t = 0
	C.CGGetActiveDisplayList(0, nil, &count)
	if count == 0 {
		return nil
	}
	cscreens := make([]C.ScreenInfo, count)
	p := (*C.ScreenInfo)(unsafe.Pointer(&cscreens[0]))
	c := int(C.GetAll(p, C.int(count)))
	var adjust zgeo.Pos
	for i := 0; i < c; i++ {
		var s Screen
		si := cscreens[i]
		s.ID = strconv.FormatInt(int64(si.sid), 10)
		s.Rect = normalizedRect(float64(si.frame.origin.x), float64(si.frame.origin.y), float64(si.frame.size.width), float64(si.frame.size.height))
		s.UsableRect = normalizedRect(float64(si.visibleFrame.origin.x), float64(si.visibleFrame.origin.y), float64(si.visibleFrame.size.width), float64(si.visibleFrame.size.height))
		s.Scale = float64(si.scale)
		s.IsMain = (si.ismain == 1)
		s.SoftScale = 1
		screens = append(screens, s)
		if s.IsMain {
			adjust = s.Rect.Pos.Negative()
		}
	}
	for i := range screens {
		screens[i].Rect.Pos.Add(adjust)
		screens[i].UsableRect.Pos.Add(adjust)
	}
	return screens
}

// SetMainScreenResolutionMin goes through the display modes of the main screen, and finds the smallest width
// one that is as big as minWidth, and sets that.
func SetMainResolutionWithinWidths(min, max zgeo.Size) {
	ms := GetMain().Rect.Size
	if max.IsNull() {
		max = min
	}
	if ms.Contains(min) && max.Contains(ms) {
		return
	}
	C.SetMainResolutionWithinWidths(C.long(min.W), C.long(min.H), C.long(max.W), C.long(max.H))
}

