package zdesktop

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// typedef struct WinIDInfo {
//     long       winID;
//     int        scale;
//     const char *err;
// } WinIDInfo;
// WinIDInfo WindowGetIDAndScaleForTitle(const char *title, long pid);
// int TerminateAppForPID(long *pid);
// int CloseWindowForTitle(const char *title, long pid);
// int SetWindowRectForTitle(const char *title, long pid, int x, int y, int w, int h);
// int ActivateWindowForTitle(const char *title, long pid);
import "C"

import (
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"
	"sync"

	"github.com/disintegration/imaging"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
)

func GetAppNameOfBrowser(btype zhttp.BrowserType, fullName bool) string {
	switch btype {
	case zhttp.Safari:
		return "Safari"
	case zhttp.Chrome:
		return "Google Chrome"
	case zhttp.Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype zhttp.BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, true)
	args = append([]string{
		"-g", // Don't bring app to foreground
		"-F", // fresh, don't open old windows. Doesn't work for safari that has been force-quit
		"-a", name,
		surl,
		"--args"}, args...)
	str, err := zprocess.RunCommand("open", 5, args...)
	if str != "" {
		zlog.Info("OpenURLInBrowser:", str)
	}
	if err != nil {
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func RunURLInBrowser(surl string, btype zhttp.BrowserType, args ...string) (*exec.Cmd, error) {
	args = append(args, surl)
	name := GetAppNameOfBrowser(btype, true)
	cmd, _, _, err := zprocess.RunApp(name, args...)
	if err != nil {
		return nil, zlog.Error(err, "RunURLInBrowser")
	}
	return cmd, err
}

func WindowWithTitleExists(title, app string) bool {
	title = getTitleWithApp(title, app)
	for _, pid := range zprocess.GetPIDsForAppName(app) {
		wInfo := C.WindowGetIDAndScaleForTitle(C.CString(title), C.long(pid))
		if int(wInfo.winID) != 0 {
			return true
		}
	}
	return false
}

func GetIDAndScaleForWindowTitle(title, app string) (id string, scale int, err error) {
	// title = getTitleWithApp(title, app)
	// fmt.Println("GetIDAndScaleForWindowTitle title, app:", title, app)
	for _, pid := range zprocess.GetPIDsForAppName(app) {
		// fmt.Println("GetIDAndScaleForWindowTitle go:", title, pid)
		w := C.WindowGetIDAndScaleForTitle(C.CString(title), C.long(pid))
		// fmt.Println("GetIDAndScaleForWindowTitle2 go:", w)
		serr := C.GoString(w.err)
		if serr != "" {
			err = errors.New(serr)
		}
		if int64(w.winID) != 0 {
			return strconv.FormatInt(int64(w.winID), 10), int(w.scale), err
		}
	}
	return
}

var screenLock sync.Mutex

func GetImageForWindowTitle(title, app string, crop zgeo.Rect, activateWindow bool) (image.Image, error) {
	filepath := zfile.CreateTempFilePath("win.png")

	// start := time.Now()
	// zlog.Info("GetImageForWindowTitle Since1:", time.Since(start))

	screenLock.Lock()
	defer screenLock.Unlock()

	// zlog.Info("GetImageForWindowTitle Since2:", time.Since(start))
	winID, scale, err := GetIDAndScaleForWindowTitle(title, app)
	if err != nil {
		return nil, zlog.Error(err, "get id scale")
	}
	if activateWindow {
		ActivateWindow(title, app)
	}
	// zlog.Info("GetImageForWindowTitle Since3:", time.Since(start))
	_, err = zprocess.RunCommand("screencapture", 0, "-o", "-x", "-l", winID, filepath) // -o is no shadow, -x is no sound, -l is window id
	if err != nil {
		return nil, zlog.Error(err, "call screen capture. id:", winID)
	}
	defer os.Remove(filepath)
	// zlog.Info("GetImageForWindowTitle Since4:", time.Since(start))
	file, err := os.Open(filepath)
	if err != nil {
		return nil, zlog.Error(err, "open", filepath)
	}
	goimage, _, err := image.Decode(file)
	if err != nil {
		return nil, zlog.Error(err, "decode", filepath)
	}
	// zlog.Info("GetImageForWindowTitle Since5:", time.Since(start))
	rect := crop.TimesD(float64(scale))
	r := image.Rect(int(rect.Min().X), int(rect.Min().Y), int(rect.Max().X), int(rect.Max().Y))
	newImage := imaging.Crop(goimage, r)
	// zlog.Info("GetImageForWindowTitle Since6:", time.Since(start))
	// TODO: set image scale
	return newImage, nil
}

func CloseWindowForTitle(title, app string) error {
	title = getTitleWithApp(title, app)
	for _, pid := range zprocess.GetPIDsForAppName(app) {
		r := C.CloseWindowForTitle(C.CString(title), C.long(pid))
		if r == 1 {
			return nil
		}
	}
	return errors.New("not found")
}

func getTitleWithApp(title, app string) string {
	if app == "Google Chrome" {
		return title + " - " + app
	}
	return title
}

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) error {
	title = getTitleWithApp(title, app)
	pids := zprocess.GetPIDsForAppName(app)
	for _, pid := range pids {
		// zlog.Info("SetWindowRectForTitle:", title, app, pid)
		r := C.SetWindowRectForTitle(C.CString(title), C.long(pid), C.int(rect.Pos.X), C.int(rect.Pos.Y), C.int(rect.Size.W), C.int(rect.Size.H))
		if r != 0 {
			return nil
		}
	}
	return errors.New("not found")
}

func SendQuitCommandToApp(app string) error {
	script := fmt.Sprintf(`quit app "%s"`, app)
	_, err := zprocess.RunAppleScript(script, 10)
	return err
}

func ActivateWindow(title, app string) {
	title = getTitleWithApp(title, app)
	pids := zprocess.GetPIDsForAppName(app)
	for _, pid := range pids {
		C.ActivateWindowForTitle(C.CString(title), C.long(pid))
	}
}
