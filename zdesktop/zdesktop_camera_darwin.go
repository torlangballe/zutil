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
// void *startCameraCaptureStream(void *stream);
// void stopCameraCaptureStream(void *stream);
// void stopCameraCaptureStream(void *stream);
// int isCameraCaptureStreamRunning(void *stream);
// int snapImageFromCaptureStream(void *stream, CGImageRef *image);
// int CanUseCamera();
import "C"

import (
	"image"
	"time"
	"unsafe"

	"github.com/torlangballe/zui/zimage"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

type CameraStream unsafe.Pointer

func StartCameraCaptureStream(cs CameraStream) CameraStream {
	return CameraStream(C.startCameraCaptureStream(unsafe.Pointer(cs)))
}

func StopCameraCaptureStream(cs CameraStream) {
	C.stopCameraCaptureStream(unsafe.Pointer(cs))
}

func IsCameraCaptureStreamRunning(cs CameraStream) bool {
	return C.isCameraCaptureStreamRunning(unsafe.Pointer(cs)) == 1
}

func CaptureCameraStreamImage(cs CameraStream, cropRect zgeo.Rect) (image.Image, error) {
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

func CanUseCamera() bool {
	return C.CanUseCamera() != 0
}

