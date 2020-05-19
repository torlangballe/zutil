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
	"syscall"

	"github.com/disintegration/imaging"
	"github.com/mitchellh/go-ps"

	"github.com/torlangballe/zutil/zcommand"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

func GetAppNameOfBrowser(btype BrowserType, fullName bool) string {
	switch btype {
	case Safari:
		return "Safari"
	case Chrome:
		return "Google Chrome"
	case Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, true)
	args = append([]string{"-F", "-g", "-a", name, surl, "--args"}, args...)
	_, err := zcommand.RunCommand("open", 5, args...)
	if err != nil {
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func RunURLInBrowser(surl string, btype BrowserType, args ...string) (*exec.Cmd, error) {
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
	for _, pid := range GetPIDsForAppName(app) {
		fmt.Println("GetIDAndScaleForWindowTitle go:", title, pid)
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
		return nil, err
	}
	_, err = zcommand.RunCommand("screencapture", 0, "-o", "-x", "-l", winID, filepath) // -o is no shadow, -x is no sound, -l is window id
	if err != nil {
		return nil, err
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

func QuitAppsByName(app string) error {
	var err error
	for _, pid := range GetPIDsForAppName(app) {
		zlog.Info("QuitAppsByName:", pid, app)
		syscall.Kill(int(-pid), syscall.SIGKILL)
	}
	return err
}

func GetPIDsForAppName(app string) []int64 {
	var pids []int64
	procs, _ := ps.Processes()
	for _, p := range procs {
		// fmt.Println("PROC:", app, "=", p.Executable())
		if p.Executable() == app {
			pids = append(pids, int64(p.Pid()))
		}
	}
	return pids
}

/*


func ResizeWindowForAppAndTitle(app, title string, size zgeo.Size) error {
	titleName := "title"
	if app == "Safari" {
		titleName = "name"
	}
	command :=
		`tell application "%s"
		activate
	    repeat with w in windows
			if %s of w is "%s" then
				set b to bounds of w
				set x to 1st item of b
				set y to 2nd item of b
				log b
				log (title of w)
				log x
				log y
				set bounds of w to {x, y, x + %d, y + %d}
			end if
		end repeat
	end tell
`
	command = fmt.Sprintf(command, app, titleName, title, int(size.W), int(size.H))
	_, err := zcommand.RunAppleScript(command, 5.0)
	// zlog.Info("ResizeAppWindowWithTitle", command, str, err)
	return err
}

func CloseUrlInBrowser(surl string, btype BrowserType) error {
	command := `
		display alert "hello"
		repeat with w in windows
			repeat with t in tabs of w
				set u to URL of t
				if u is equal to "%s" then
					close t
					return
				end if
           end repeat
       end repeat
	`
	command = fmt.Sprintf(command, surl)
	_, err := RunAppleScript(command, 10.0)
	return err
}

func ResizeBrowserWindowWithTitle(btype BrowserType, title string, rect zgeo.Rect) error {
	titleName := "title"
	if btype == Safari {
		titleName = "name"
	}
	app := GetAppNameOfBrowser(btype, true)
	command :=
		`tell application "%s"
		activate
		set bounds of every window whose %s is "%s" to {%g,%g,%g,%g}
		end tell
`
	command = fmt.Sprintf(command, app, titleName, title, rect.Min().X, rect.Min().Y, rect.Max().X, rect.Max().Y)
	// zlog.Info("CloseWindowWithTitle:", app, title, "\n", command)
	_, err := RunAppleScript(command, 5.0)
	//	zlog.Info("CloseWindowWithTitle done", err)
	return err
}

*/
