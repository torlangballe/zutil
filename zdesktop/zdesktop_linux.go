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
		return "chromium-browser"
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

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) (winPID int64, err error) {
	return 0, nil
}

func GetImageForWindowTitle(title, app string, oldPID int64, insetRect zgeo.Rect) (img image.Image, pid int64, err error) {
	return nil, 0, nil
}
