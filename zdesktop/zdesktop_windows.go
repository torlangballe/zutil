package zdesktop

import (
	"image"
	"sync"
	"syscall"

	"github.com/AllenDang/w32"
	"github.com/kbinani/screenshot"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

func GetAppNameOfBrowser(btype zhttp.BrowserType, fullName bool) string {
	switch btype {
	case zhttp.Safari:
		zlog.Fatal(nil)

	case zhttp.Chrome:
		if fullName {
			return "Google Chrome"
		}
		return "chrome"
	case zhttp.Edge:
		if fullName {
			return "Microsoft Edge"
		}
		return "edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype zhttp.BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, false)
	//	_, err := zprocess.RunCommand("start", 5, args...)
	// zlog.Info("*********** OpenURLInBrowser:", surl)
	//	zlog.Info("OpenUrlInBrowser:", name, btype, args)

	args = append([]string{"/c", "start", name, surl}, args...)
	_, err := zprocess.RunCommand("cmd", 4, args...)
	if err != nil {
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func WindowWithTitleExists(title, app string) bool {
	handle := getWindowHandleFromTitle(title, app)
	return handle != 0
}

func getWindows(hwnd syscall.Handle, lparam uintptr) bool {
	return true
}

var getWindowLock sync.Mutex
var getWindowTitle string
var getWindowHandle w32.HWND

func makeWindowCallback() uintptr {
	cb := func(hwnd syscall.Handle, lparam uintptr) int {
		h := w32.HWND(hwnd)
		wtitle := w32.GetWindowText(h)
		if wtitle == getWindowTitle {
			// zlog.Info("WIN:", wtitle, h)
			getWindowHandle = h
			return 0
		}
		return 1
	}
	return syscall.NewCallback(cb)
}

var moduser32 = syscall.NewLazyDLL("user32.dll")
var procEnumWindows = moduser32.NewProc("EnumWindows")
var winCB = makeWindowCallback()

func getWindowHandleFromTitle(title, app string) w32.HWND {
	getWindowLock.Lock()
	defer getWindowLock.Unlock()
	getWindowTitle = title + " - " + app
	getWindowHandle = 0
	// zlog.Info("getWindowHandleFromTitle:", getWindowTitle)
	r1, _, e1 := syscall.Syscall(procEnumWindows.Addr(), 2, winCB, 0, 0)
	if r1 != 0 {
		if e1 != 0 {
			zlog.Error(error(e1), "enum windows")
			return 0
		}
	}
	return getWindowHandle
}

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) error {
	// fmt.Println("SetWindowSizeForTitle:", title)
	h := getWindowHandleFromTitle(title, app)
	if h != 0 {
		w32.MoveWindow(h, int(rect.Pos.X), int(rect.Pos.Y), int(rect.Size.W), int(rect.Size.H), true)
		return nil
	}
	return zlog.Error(nil, "no window", title)
}

var screenLock sync.Mutex

func GetImageForWindowTitle(title, app string, crop zgeo.Rect) (image.Image, error) {
	screenLock.Lock()
	ActivateWindow(title, app)
	bounds := image.Rect(int(crop.Min().X), int(crop.Min().Y), int(crop.Max().X), int(crop.Max().Y))
	nimage, err := screenshot.CaptureRect(bounds)
	screenLock.Unlock()
	zlog.Info("IMAGE:", bounds, err)
	if err != nil {
		return nil, zlog.Error(err, "capture rect")
	}
	return nimage, nil
}

func ActivateWindow(title, app string) {
	h := getWindowHandleFromTitle(title, app)
	zlog.Info("activate:", title, app, h)
	if h != 0 {
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

func SendQuitCommandToApp(app string) error {
	return nil
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
// 	_, err := zprocess.RunCommand("cmd", 4, args...)
// 	zlog.Info("TerminateAppsByName:", args, err)
// 	return err
// }
