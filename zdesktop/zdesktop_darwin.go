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
// int SetWindowSizeForTitle(const char *title, long pid, int w, int h);
import "C"

import (
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"strconv"

	"github.com/disintegration/imaging"

	"github.com/torlangballe/zutil/uhttp"
	"github.com/torlangballe/zutil/zcommand"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

func GetAppNameOfBrowser(btype uhttp.BrowserType, fullName bool) string {
	switch btype {
	case uhttp.Safari:
		return "Safari"
	case uhttp.Chrome:
		return "Google Chrome"
	case uhttp.Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype uhttp.BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, true)
	args = append([]string{
		"-g", // Don't bring app to foreground
		"-F", // fresh, don't open old windows. Doesn't work for safari that has been force-quit
		"-a", name,
		surl,
		"--args"}, args...)
	_, err := zcommand.RunCommand("open", 5, args...)
	if err != nil {
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func RunURLInBrowser(surl string, btype uhttp.BrowserType, args ...string) (*exec.Cmd, error) {
	args = append(args, surl)
	name := GetAppNameOfBrowser(btype, true)
	cmd, _, _, err := zcommand.RunApp(name, args...)
	if err != nil {
		return nil, zlog.Error(err, "RunURLInBrowser")
	}
	return cmd, err
}

func WindowWithTitleExists(title, app string) bool {
	title = getTitleWithApp(title, app)
	for _, pid := range GetPIDsForAppName(app) {
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
	for _, pid := range GetPIDsForAppName(app) {
		// fmt.Println("GetIDAndScaleForWindowTitle go:", title, pid)
		w := C.WindowGetIDAndScaleForTitle(C.CString(title), C.long(pid))
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

func GetImageForWindowTitle(title, app string, crop zgeo.Rect) (image.Image, error) {
	_, filepath, err := zfile.CreateTempFile("win.png")
	zlog.Assert(err == nil)

	winID, scale, err := GetIDAndScaleForWindowTitle(title, app)
	if err != nil {
		return nil, zlog.Error(err, "get id scale")
	}
	_, err = zcommand.RunCommand("screencapture", 0, "-o", "-x", "-l", winID, filepath) // -o is no shadow, -x is no sound, -l is window id
	if err != nil {
		return nil, zlog.Error(err, "call screen capture", winID)
	}
	file, err := os.Open(filepath)
	if err != nil {
		return nil, zlog.Error(err, "open", filepath)
	}
	goimage, _, err := image.Decode(file)
	if err != nil {
		return nil, zlog.Error(err, "decode", filepath)
	}
	rect := crop.TimesD(float64(scale))
	r := image.Rect(int(rect.Min().X), int(rect.Min().Y), int(rect.Max().X), int(rect.Max().Y))
	newImage := imaging.Crop(goimage, r)
	// TODO: set image scale
	return newImage, nil
}

func CloseWindowForTitle(title, app string) error {
	title = getTitleWithApp(title, app)
	for _, pid := range GetPIDsForAppName(app) {
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

func SetWindowSizeForTitle(title, app string, size zgeo.Size) error {
	title = getTitleWithApp(title, app)
	pids := GetPIDsForAppName(app)
	for _, pid := range pids {
		r := C.SetWindowSizeForTitle(C.CString(title), C.long(pid), C.int(size.W), C.int(size.H))
		if r != 0 {
			return nil
		}
	}
	return errors.New("not found")
}

func SendQuitCommandToApp(app string) error {
	script := fmt.Sprintf(`quit app "%s"`, app)
	_, err := zcommand.RunAppleScript(script, 10)
	return err
}
