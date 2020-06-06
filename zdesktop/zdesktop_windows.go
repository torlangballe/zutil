package zdesktop

import (
	"image"
	"sync"
	"syscall"

	"github.com/AllenDang/w32"
	"github.com/hnakamur/w32syscall"
	"github.com/kbinani/screenshot"
	"github.com/torlangballe/zutil/uhttp"
	"github.com/torlangballe/zutil/zcommand"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

func GetAppNameOfBrowser(btype uhttp.BrowserType, fullName bool) string {
	switch btype {
	case uhttp.Safari:
		zlog.Fatal(nil)

	case uhttp.Chrome:
		if fullName {
			return "Google Chrome"
		}
		return "chrome"
	case uhttp.Edge:
		if fullName {
			return "Microsoft Edge"
		}
		return "edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype uhttp.BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, false)
	//	_, err := zcommand.RunCommand("start", 5, args...)
	// zlog.Info("*********** OpenURLInBrowser:", surl)
	//	zlog.Info("OpenUrlInBrowser:", name, btype, args)

	args = append([]string{"/c", "start", name, surl}, args...)
	_, err := zcommand.RunCommand("cmd", 4, args...)
	if err != nil {
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func WindowWithTitleExists(title, app string) bool {
	zlog.Fatal(nil, "not implemented")
	return false
}

var getWindowLock sync.Mutex
var getWindowTitle string
var getWindowHandle w32.HWND

func getWindows(hwnd syscall.Handle, lparam uintptr) bool {
	h := w32.HWND(hwnd)
	title := w32.GetWindowText(h)
	// if strings.Contains(title, "Google") {
	// 	fmt.Println("WIN:", title)
	// }
	if title == getWindowTitle {
		getWindowHandle = h
		return false
	}
	return true
}

func getWindowHandleFromTitle(title, app string) w32.HWND {
	getWindowLock.Lock()
	getWindowTitle = title + " - " + app
	getWindowHandle = 0
	// zlog.Info("getWindowHandleFromTitle:", getWindowTitle)
	err := w32syscall.EnumWindows(getWindows, 0)
	if err != nil {
		zlog.Error(err, "enum windows")
	}
	// Doing this with fixed function and global variables to avoid "fatal error: too many callback functions" in windows enumerating
	getWindowLock.Unlock()
	return getWindowHandle
}

func SetWindowSizeForTitle(title, app string, size zgeo.Size) error {
	// fmt.Println("SetWindowSizeForTitle:", title)
	h := getWindowHandleFromTitle(title, app)
	if h != 0 {
		w32.MoveWindow(h, -7, 0, int(size.W), int(size.H), true)
		// w32.SetWindowPos(h, 0, -7, 0, int(size.W), int(size.H), 0x400)
		return nil
	}
	return zlog.Error(nil, "no window", title)
}

func GetImageForWindowTitle(title, app string, crop zgeo.Rect) (image.Image, error) {
	ActivateWindow(title, app)
	bounds := image.Rectangle{}
	bounds.Min.X = int(crop.Min().X)
	bounds.Min.Y = int(crop.Min().Y)
	bounds.Max.X = int(crop.Max().X)
	bounds.Max.Y = int(crop.Max().Y)
	nimage, err := screenshot.CaptureRect(bounds)
	if err != nil {
		return nil, zlog.Error(err, "capture rect")
	}
	return nimage, nil
}

func ActivateWindow(title, app string) {
	h := getWindowHandleFromTitle(title, app)
	if h != 0 {
		// zlosnipg.Info("activate:", title, h)
		w32.SetForegroundWindow(h)
	}
}

func CloseWindowForTitle(title, app string) error {
	const WM_CLOSE = 0x10

	h := getWindowHandleFromTitle(title, app)
	if h != 0 {
		ok := w32.PostMessage(h, WM_CLOSE, 0, 0)
		//		ok := w32.DestroyWindow(h)
		zlog.Info("CloseWindowForTitle:", ok, title, h)
		return nil
	}
	return zlog.Error(nil, "no window", title)
}

// func TerminateAppsByName(name string, force, children bool) error {
// 	args := []string{"/c", "taskkill"}
// 	if force {
// 		args = append(args, "/F")
// 	}
// 	if children {
// 		args = append(args, "/T")
// 	}
// 	args = append(args, "/IM", name+".exe")
// 	_, err := zcommand.RunCommand("cmd", 4, args...)
// 	zlog.Info("TerminateAppsByName:", args, err)
// 	return err
// }
