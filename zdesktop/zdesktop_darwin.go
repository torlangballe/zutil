package zdesktop

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// #cgo LDFLAGS: -framework CoreGraphics
// #cgo LDFLAGS: -framework AVFoundation
// #cgo LDFLAGS: -framework CoreMedia
// #cgo LDFLAGS: -framework ScreenCaptureKit
// #include <CoreFoundation/CoreFoundation.h>
// #include <CoreGraphics/CoreGraphics.h>
// typedef struct WinIDInfo {
//     long       winID;
//     int        scale;
//     const char *err;
// 	   int        x;
//     int        y;
//     int        w;
//     int        h;
// } WinIDInfo;
// WinIDInfo WindowGetIDScaleAndRectForTitle(const char *title, long pid);
// int TerminateAppForPID(long *pid);
// int CloseWindowForTitle(const char *title, long pid);
// int SetWindowRectForTitle(const char *title, long pid, int x, int y, int w, int h);
// int ActivateWindowForTitle(const char *title, const char *bundleID, int activateApp);
// void ConvertARGBToRGBAOpaque(int w, int h, int stride, unsigned char *img);
// int canControlComputer(int prompt);
// int GetWindowCountForPID(long pid);
// int CanRecordScreen();
// int CanUseCamera();
// void PrintWindowTitles();
// const char *GetAllWindowTitlesTabSeparated(long forPid);
// typedef struct Image {
//   int width;
//   int height;
//   char *data;
// } Image;
// void ShowAlert(char *str);
// int CloseOldWindowWithSamePIDAndRectOnceNew(long pid, int x, int y, int w, int h);
// void CloseOldWindowWithSamePIDAndRect(long pid, int x, int y, int w, int h);
// const char *ImageOfWindow(const char *winTitle, const char *appBundleID, int displayID, CGRect cropRect, CGImageRef *cgImage);
// void *startCameraCaptureStream(void *stream);
// void stopCameraCaptureStream(void *stream);
// void stopCameraCaptureStream(void *stream);
// int isCameraCaptureStreamRunning(void *stream);
// int snapImageFromCaptureStream(void *stream, CGImageRef *image);
import "C"

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
)

var pidCacheLock sync.Mutex
var pidCache = map[string]int64{}

func ClearAppPIDCache() {
	pidCacheLock.Lock()
	pidCache = map[string]int64{}
	pidCacheLock.Unlock()
}

func PrintWindowTitles() {
	// zlog.Info("PrintWindowTitles")
	C.PrintWindowTitles()
}

func GetCachedPIDForAppName(app string) (int64, error) {
	var pid int64
	// pidCacheLock.Lock()
	// pid, got := pidCache[app]
	// pidCacheLock.Unlock()
	// if !got { // We force new get until we have something for that app...
	pids := zprocess.GetPIDsForAppName(app, false)
	if len(pids) == 0 {
		return 0, zlog.NewError(nil, "no pid for", app)
	}
	pidCacheLock.Lock()
	pid = pids[0]
	pidCache[app] = pid
	pidCacheLock.Unlock()
	return pid, nil
}

func GetAppIDOfBrowser(btype zdevice.BrowserType) string {
	switch btype {
	case zdevice.Safari:
		return "com.apple.Safari"
	case zdevice.Chrome:
		return "com.google.Chrome"
	case zdevice.Edge:
		return "com.microsoft.edgemac.Canary"
	}
	return ""
}

func GetAppNameOfBrowser(btype zdevice.BrowserType, fullName bool) string {
	switch btype {
	case zdevice.Safari:
		return "Safari"
	case zdevice.Chrome:
		return "Google Chrome"
	case zdevice.Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func OpenURLInBrowser(surl string, btype zdevice.BrowserType, args ...any) error {
	name := GetAppNameOfBrowser(btype, true)
	args = append([]any{
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
		return zlog.Error("OpenURLInBrowser", err)
	}
	return err
}

func RunURLInBrowser(surl string, btype zdevice.BrowserType, args ...any) (*exec.Cmd, error) {
	args = append(args, surl)
	name := GetAppNameOfBrowser(btype, true)
	cmd, _, _, _, err := zprocess.RunApp(name, nil, args...)
	if err != nil {
		return nil, zlog.Error("RunURLInBrowser", err)
	}
	return cmd, err
}

func WindowWithTitleExists(title, app string) bool {
	//	title = getTitleWithApp(title, app)
	pid, _ := GetCachedPIDForAppName(app)
	if pid != 0 {
		ctitle := C.CString(title)
		wInfo := C.WindowGetIDScaleAndRectForTitle(ctitle, C.long(pid))
		C.free(unsafe.Pointer(ctitle))
		if int(wInfo.winID) != 0 {
			return true
		}
	}
	return false
}

func GetIDScaleAndRectForWindowTitle(title, app string, pid int64) (id string, scale int, cropMargins zgeo.Rect, pidOut int64, err error) {
	// fmt.Println("GetIDAndScaleForWindowTitle title, app:", title, app)
	pids := []int64{pid}
	if pid == 0 {
		pids = zprocess.GetPIDsForAppName(app, false)
	}
	// pid, _ := GetCachedPIDForAppName(app)
	// fmt.Println("SetWindowRectForTitle:", title, app, pids)
	for _, pid := range pids {
		// fmt.Println("GetIDAndScaleForWindowTitle go:", title, pid)
		ctitle := C.CString(title)
		w := C.WindowGetIDScaleAndRectForTitle(ctitle, C.long(pid)) // w.err is a const if anything, so no need to free
		C.free(unsafe.Pointer(ctitle))
		serr := C.GoString(w.err)
		// fmt.Println("GetIDAndScaleForWindowTitle2 go:", serr, w.winID)
		if serr != "" {
			err = errors.New(serr)
			continue
		}
		if int64(w.winID) != 0 {
			r := zgeo.RectFromXYWH(float64(w.x), float64(w.y), float64(w.w), float64(w.h))
			return strconv.FormatInt(int64(w.winID), 10), int(w.scale), r, pid, err
		}
	}
	return
}

func CloseOldWindowWithSamePIDAndRectOnceNew(pid int64, r zgeo.Rect) bool {
	n := C.CloseOldWindowWithSamePIDAndRectOnceNew(C.long(pid), C.int(r.Pos.X), C.int(r.Pos.Y), C.int(r.Size.W), C.int(r.Size.H))
	zlog.Info("CloseOldWindowWithIDInRectOnceNew:", n)
	return n != 0
}

func CloseOldWindowWithSamePIDAndRect(pid int64, r zgeo.Rect) {
	C.CloseOldWindowWithSamePIDAndRect(C.long(pid), C.int(r.Pos.X), C.int(r.Pos.Y), C.int(r.Size.W), C.int(r.Size.H))
}

func CloseWindowForTitle(title, app string) error {
	pids := zprocess.GetPIDsForAppName(app, false)
	for _, pid := range pids {
		ctitle := C.CString(title)
		r := C.CloseWindowForTitle(ctitle, C.long(pid))
		C.free(unsafe.Pointer(ctitle))
		if r == 1 {
			return nil
		}
	}
	return errors.New("window not found for closing")
}

func SetWindowRectForTitle(title, app string, rect zgeo.Rect) (winPID int64, err error) {
	pids := zprocess.GetPIDsForAppName(app, false)
	for _, pid := range pids {
		ctitle := C.CString(title)
		r := C.SetWindowRectForTitle(ctitle, C.long(pid), C.int(rect.Pos.X), C.int(rect.Pos.Y), C.int(rect.Size.W), C.int(rect.Size.H))
		C.free(unsafe.Pointer(ctitle))
		if r != 0 {
			return pid, nil
		}
	}
	return 0, errors.New("Not Found")
}

func SendQuitCommandToApp(app string) error {
	script := fmt.Sprintf(`quit app "%s"`, app)
	_, err := zprocess.RunAppleScript(script, 10)
	return err
}

func MakeWindowFrontmost(title, appID string, activateApp bool) error {
	//	title = getTitleWithApp(title, app)
	// pid, _ := GetCachedPIDForAppName(app)
	// if pid == 0 {
	// 	return errors.New("No app found")
	// }
	ctitle := C.CString(title)
	cappID := C.CString(appID)
	activate := C.int(0)
	if activateApp {
		activate = 1
	}
	r := C.ActivateWindowForTitle(ctitle, cappID, activate)
	if r == 0 {
		return errors.New("Window not found")
	}
	C.free(unsafe.Pointer(ctitle))
	C.free(unsafe.Pointer(cappID))
	return nil
}

func AddExecutableToLoginItems(exePath, name string, hidden bool) error {
	command := `tell application "System Events" to make new login item at end with properties {path:"%s", name:"%s", hidden:%v}`
	command = fmt.Sprintf(command, exePath, name, hidden)
	str, err := zprocess.RunAppleScript(command, 10)
	if err != nil {
		return zlog.Error("🟨error adding executable", exePath, "to login items:", str, err)
	}
	return nil
}

func CanControlComputer(prompt bool) bool {
	p := 0
	if prompt {
		p = 1
	}
	return C.canControlComputer(C.int(p)) == 1
}

func CanGetWindowInfo() bool {
	pid, err := GetCachedPIDForAppName("Finder")
	if err != nil {
		return false
	}
	return C.GetWindowCountForPID(C.long(pid)) != -1
}

func CanUseCamera() bool {
	return C.CanUseCamera() != 0
}

func GetWindowCountForPid(pid int64) int {
	return int(C.GetWindowCountForPID(C.long(pid)))
}

func CanRecordScreen() bool {
	return C.CanRecordScreen() == 1
}

func GetAllWindowTitlesForApp(app string) []string {
	pid, _ := GetCachedPIDForAppName(app)
	if pid == 0 {
		return nil
	}
	ctitles := C.GetAllWindowTitlesTabSeparated(C.long(pid))
	stitles := C.GoString(ctitles)
	C.free(unsafe.Pointer(ctitles))
	if len(stitles) == 0 {
		return nil
	}
	// zlog.Info("GetAllWindowTitlesForApp", app, str)
	titles := strings.Split(stitles, "\t")
	return titles
}

func ShowAlert(str string) {
	cstr := C.CString(str)
	C.ShowAlert(cstr)
	C.free(unsafe.Pointer(cstr))
}

var captureLock sync.Mutex

func frontWindowAndGetImageFromDisplay(title, appID string, contentCropRectInDisplay zgeo.Rect, croppingFromDisplayID string) (image.Image, error) {
	err := MakeWindowFrontmost(title, appID, true)
	time.Sleep(time.Millisecond * 100)
	if err != nil {
		return nil, err
	}
	return GetImageForDisplay(croppingFromDisplayID, contentCropRectInDisplay)
}

func GetImageForWindowTitle(title, appID string, cropRect zgeo.Rect, croppingFromDisplayID string) (image.Image, error) {
	captureLock.Lock()
	defer captureLock.Unlock()
	if croppingFromDisplayID != "" {
		return frontWindowAndGetImageFromDisplay(title, appID, cropRect, croppingFromDisplayID)
	}
	return getImageForWindowOrDisplay(title, appID, "", cropRect)
}

func GetImageForDisplay(displayID string, cropInsetRect zgeo.Rect) (image.Image, error) {
	zlog.Info("GetImageForDisplay:", displayID, cropInsetRect)
	return getImageForWindowOrDisplay("", "", displayID, cropInsetRect)
}

func getImageForWindowOrDisplay(title, appID, displayID string, cropInsetRect zgeo.Rect) (image.Image, error) {
	var cgrect C.CGRect
	ctitle := C.CString(title)
	cappid := C.CString(appID)
	cgrect.origin.x = C.CGFloat(cropInsetRect.Pos.X)
	cgrect.origin.y = C.CGFloat(cropInsetRect.Pos.Y)
	cgrect.size.width = C.CGFloat(cropInsetRect.Size.W)
	cgrect.size.height = C.CGFloat(cropInsetRect.Size.H)
	var cgImage C.CGImageRef = C.CGImageRef(C.NULL)

	displayInt, _ := strconv.Atoi(displayID)
	// zlog.Warn("GetImageForWindowTitle:", title)
	// start := time.Now()//
	cerr := C.ImageOfWindow(ctitle, cappid, C.int(displayInt), cgrect, &cgImage)
	serr := C.GoString(cerr)

	if serr != "" || cgImage == C.CGImageRef(C.NULL) {
		if serr == "timed out" {
			zlog.Info("GetImageForWindowTitle timed out:", title)
		}
		if serr == "" {
			return nil, nil
		}
		return nil, zlog.Error(serr, title, appID)
	}
	image, err := zimage.CGImageToGoImage(unsafe.Pointer(cgImage), zgeo.Rect{}, 1)
	// zlog.Warn("GetImageForWindowTitle Done:", image != nil, err)
	C.CGImageRelease(cgImage)
	return image, err
}

type CameraStreamType unsafe.Pointer

func StartCameraCaptureStream(cs CameraStreamType) CameraStreamType {
	return CameraStreamType(C.startCameraCaptureStream(unsafe.Pointer(cs)))
}

func StopCameraCaptureStream(cs CameraStreamType) {
	C.stopCameraCaptureStream(unsafe.Pointer(cs))
}

func IsCameraCaptureStreamRunning(cs CameraStreamType) bool {
	return C.isCameraCaptureStreamRunning(unsafe.Pointer(cs)) == 1
}

func CaptureCameraStreamImage(cs CameraStreamType, cropRect zgeo.Rect) (image.Image, error) {
	var cgImage C.CGImageRef
	now := time.Now()
	sleepMS := time.Duration(8)
	count := 0
	for {
		timedOut := (time.Since(now) > time.Second)
		// clearWantIfFail := C.int(0)
		// if timedOut {
		// 	clearWantIfFail = 1
		// }
		r := C.snapImageFromCaptureStream(unsafe.Pointer(cs), &cgImage) // clearWantIfFail
		count++
		if r != 0 {
			img, err := zimage.CGImageToGoImage(unsafe.Pointer(cgImage), cropRect, 1)
			C.CGImageRelease(cgImage)
			zlog.Info("CaptureCameraStreamImage:", time.Since(now), img.Bounds(), cropRect, count)
			return img, err
		}
		if timedOut {
			return nil, zlog.NewError("No imaged captured in time interval", count)
		}
		time.Sleep(time.Millisecond * sleepMS)
		if sleepMS > 1 {
			sleepMS /= 2
		}
	}
}
