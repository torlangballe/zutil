package zdesktop

import (
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
)

func GetAppNameOfBrowser(btype zdevice.BrowserType, fullName bool) string {
	switch btype {
	case zdevice.Safari:
		return "Safari"
	case zdevice.Chrome:
		return "chromium-browser"
	case zdevice.Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func CloseOldWindowWithSamePIDAndRect(pid int64, r zgeo.Rect) {}
func CloseOldWindowWithSamePIDAndRectOnceNew(pid int64, r zgeo.Rect) bool {
	return true
}
