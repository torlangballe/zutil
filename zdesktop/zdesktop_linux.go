package zdesktop

import (
	"image"

	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
)

func GetAppNameOfBrowser(btype zdevice.BrowserType, fullName bool) string {
	switch btype {
	case zdevice.Safari:
		return "Safari"
	case zdevice.Chrome:
		return "google-chrome"
	case zdevice.Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func CloseOldWindowWithSamePIDAndRect(pid int64, r zgeo.Rect)                    {}
func CloseOldWindowWithSamePIDAndRectOnceNew(pid int64, r zgeo.Rect) bool        { return true }
func OpenURLInBrowser(surl string, btype zdevice.BrowserType, args ...any) error { return nil }
func SendQuitCommandToApp(app string) error                                      { return nil }
func PrintWindowTitles()                                                         {}
func CloseWindowForTitle(title, app string) error                                { return nil }
func CanGetWindowInfo() bool                                                     { return true }
func CanRecordScreen() bool                                                      { return true }
func CanControlComputer(prompt bool) bool                                        { return true }
func GetAllWindowTitlesForApp(app string) []string                               { return nil }
func ShowAlert(str string)                                                       {}
func MakeWindowFrontmost(title, appID string, activateApp bool) error            { return nil }

func GetImageForDisplay(displayID string, cropInsetRect zgeo.Rect) (image.Image, error) {
	return nil, nil
}

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) (winPID int64, err error) {
	return 0, nil
}

func GetImageForWindowTitle(title, appID string, preflightOnly bool, cropRect zgeo.Rect, croppingFromDisplayID string) (image.Image, error) {
	return nil, nil
}

func GetAppIDOfBrowser(btype zdevice.BrowserType) string {
	return ""
}
