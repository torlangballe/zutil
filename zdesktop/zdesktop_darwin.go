package zdesktop

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// #cgo LDFLAGS: -framework CoreGraphics
// #include <CoreGraphics/CoreGraphics.h>
// #include <stdlib.h>
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
// int ActivateWindowForTitle(const char *title, long pid);
// void ConvertARGBToRGBAOpaque(int w, int h, int stride, unsigned char *img);
// void CloseWindowsForPIDIfNotInTitles(int pid, char *stitles);
// int canControlComputer(int prompt);
// int getWindowCountForPID(long pid);
// int canRecordScreen();
// void printWindowTitles();
// const char *getAllWindowTitlesTabSeparated(long forPid);
// typedef struct Image {
//   int width;
//   int height;
//   char *data;
// } Image;
// CGImageRef GetWindowImage(long winID);
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
	C.printWindowTitles()
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
		return zlog.Error(err, "OpenURLInBrowser")
	}
	return err
}

func RunURLInBrowser(surl string, btype zdevice.BrowserType, args ...any) (*exec.Cmd, error) {
	args = append(args, surl)
	name := GetAppNameOfBrowser(btype, true)
	cmd, _, _, _, err := zprocess.RunApp(name, args...)
	if err != nil {
		return nil, zlog.Error(err, "RunURLInBrowser")
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
		w := C.WindowGetIDScaleAndRectForTitle(ctitle, C.long(pid))
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

func GetImageForWindowTitle(title, app string, oldPID int64, insetRect zgeo.Rect) (img image.Image, pid int64, err error) {
	winID, _, _, pid, err := GetIDScaleAndRectForWindowTitle(title, app, oldPID)
	fmt.Println("GetImageForWindowTitle:", winID, err, "pid:", pid, "oldpid:", oldPID, title, app, zprocess.GetPIDsForAppName(app, false))
	if err != nil {
		return nil, 0, zlog.Error(err, "get id scale")
	}
	img, err = GetWindowImage(winID, insetRect)
	return img, pid, err
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
	return 0, errors.New("not found")
}

func SendQuitCommandToApp(app string) error {
	script := fmt.Sprintf(`quit app "%s"`, app)
	_, err := zprocess.RunAppleScript(script, 10)
	return err
}

func ActivateWindow(title, app string) {
	//	title = getTitleWithApp(title, app)
	pid, _ := GetCachedPIDForAppName(app)
	if pid != 0 {
		ctitle := C.CString(title)
		C.ActivateWindowForTitle(ctitle, C.long(pid))
		C.free(unsafe.Pointer(ctitle))
	}
}

func AddExecutableToLoginItems(exePath, name string, hidden bool) error {
	command := `tell application "System Events" to make new login item at end with properties {path:"%s", name:"%s", hidden:%v}`
	command = fmt.Sprintf(command, exePath, name, hidden)
	str, err := zprocess.RunAppleScript(command, 10)
	if err != nil {
		return zlog.Error(err, "🟨error adding executable", exePath, "to login items:", str)
	}
	return nil
}

func createColorspace() C.CGColorSpaceRef {
	return C.CGColorSpaceCreateWithName(C.kCGColorSpaceSRGB)
}

func createBitmapContext(width int, height int, data *C.uint32_t, bytesPerRow int) C.CGContextRef {
	colorSpace := createColorspace()
	if colorSpace == 0 {
		return 0
	}
	defer C.CGColorSpaceRelease(colorSpace)

	return C.CGBitmapContextCreate(unsafe.Pointer(data),
		C.size_t(width),
		C.size_t(height),
		8, // bits per component
		C.size_t(bytesPerRow),
		colorSpace,
		C.kCGImageAlphaNoneSkipFirst)
}

func CGImageToGoImage(cgimage C.CGImageRef, insetRect zgeo.Rect) (image.Image, error) {
	var cw, ch int
	iw := int(C.CGImageGetWidth(cgimage))
	ih := int(C.CGImageGetHeight(cgimage))
	cw = iw
	ch = ih
	if !insetRect.IsNull() {
		cw = int(insetRect.Size.W)
		ch = int(insetRect.Size.H)
	}
	img := image.NewNRGBA(image.Rect(0, 0, cw, ch))
	if img == nil {
		return nil, zlog.Error(nil, "NewRGBA returned nil", cw, ch)
	}
	// zlog.Info("THUMB insetRect:", insetRect)
	ctx := createBitmapContext(cw, ch, (*C.uint32_t)(unsafe.Pointer(&img.Pix[0])), img.Stride)
	diff := float64(ih - ch)
	x := C.CGFloat(-insetRect.Pos.X)
	y := C.CGFloat(-diff + insetRect.Pos.Y)
	cgDrawRect := C.CGRectMake(x, y, C.CGFloat(iw), C.CGFloat(ih))
	C.CGContextDrawImage(ctx, cgDrawRect, cgimage)

	C.ConvertARGBToRGBAOpaque(C.int(cw), C.int(ch), C.int(img.Stride), (*C.uchar)(unsafe.Pointer(&img.Pix[0])))
	C.CGContextRelease(ctx)

	return img, nil
}

func GetWindowImage(winID string, insetRect zgeo.Rect) (image.Image, error) {
	wid, _ := strconv.ParseInt(winID, 10, 64)
	if wid == 0 {
		return nil, zlog.Error(nil, "no valid image id")
	}
	zlog.Assert(wid != 0)
	start := time.Now()
	cgimage := C.GetWindowImage(C.long(wid))
	if cgimage == C.CGImageRef(0) {
		err := zlog.Error(nil, "get window image returned nil", time.Since(start), "wid:", wid)
		PrintWindowTitles()
		return nil, err

	}
	// zlog.Info("GetWindowImage:", time.Since(start))
	img, err := CGImageToGoImage(cgimage, insetRect)
	// zlog.Info("GetWindowImage Make Go Image:", time.Since(start))
	C.CGImageRelease(cgimage)
	return img, err
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
	return C.getWindowCountForPID(C.long(pid)) != -1
}

func GetWindowCountForPid(pid int64) int {
	return int(C.getWindowCountForPID(C.long(pid)))
}

func CanRecordScreen() bool {
	return C.canRecordScreen() == 1
}

func GetAllWindowTitlesForApp(app string) []string {
	pid, _ := GetCachedPIDForAppName(app)
	if pid == 0 {
		return nil
	}
	str := C.GoString(C.getAllWindowTitlesTabSeparated(C.long(pid)))
	if len(str) == 0 {
		return nil
	}
	// zlog.Info("GetAllWindowTitlesForApp", app, str)
	return strings.Split(str, "\t")
}
