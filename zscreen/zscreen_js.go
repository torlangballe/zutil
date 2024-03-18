package zscreen

import (
	"math"
	"syscall/js"

	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
)

var printed bool

func GetAll() []Screen {
	var m Screen

	win := js.Global().Get("window")
	s := win.Get("screen")
	w := s.Get("width").Float()
	h := s.Get("height").Float()

	dpr := win.Get("devicePixelRatio").Float()
	m.Rect = zgeo.RectMake(0, 0, w, h)
	m.Scale = math.Round(dpr)
	m.ID = "1"
	if zdevice.OS() == zdevice.MacOSType {
		// zlog.Info("SCALE:", m.Scale, win.Get("screen").Get("height").Float(), win.Get("devicePixelRatio").Float())
		m.Scale = 2
	}
	m.IsMain = true
	m.SoftScale = 1
	// m.UsableRect = m.Rect

	return []Screen{m}
}

/*
check for touch screen:
var hasTouchScreen = false;
if ("maxTouchPoints" in navigator) {
    hasTouchScreen = navigator.maxTouchPoints > 0;
} else if ("msMaxTouchPoints" in navigator) {
    hasTouchScreen = navigator.msMaxTouchPoints > 0;
} else {
    var mQ = window.matchMedia && matchMedia("(pointer:coarse)");
    if (mQ && mQ.media === "(pointer:coarse)") {
        hasTouchScreen = !!mQ.matches;
    } else if ('orientation' in window) {
        hasTouchScreen = true; // deprecated, but good fallback
    } else {
        // Only as a last resort, fall back to user agent sniffing
        var UA = navigator.userAgent;
        hasTouchScreen = (
            /\b(BlackBerry|webOS|iPhone|IEMobile)\b/i.test(UA) ||
            /\b(Android|Windows Phone|iPad|iPod)\b/i.test(UA)
        );
    }
}
if (hasTouchScreen)
    document.getElementById("exampleButton").style.padding="1em";

*/
