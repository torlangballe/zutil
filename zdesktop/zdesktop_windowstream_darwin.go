package zdesktop

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// #cgo LDFLAGS: -framework CoreGraphics
// #cgo LDFLAGS: -framework AVFoundation
// #cgo LDFLAGS: -framework CoreMedia
// #cgo LDFLAGS: -framework ScreenCaptureKit
// #cgo LDFLAGS: -framework VideoToolbox
// #cgo LDFLAGS: -framework AudioToolbox
// #include <stdlib.h>
// #include "zdesktop_windowstream_darwin.h"
/*
void *startWindowCaptureStreamWithRegion(const char *winTitle, const char *appBundleID, int captureAudio, int srcOffsetX, int srcOffsetY, int srcWidth, int srcHeight, int destWidth, int destHeight, char *err, int errLen);
*/
import "C"

import (
	"errors"
	"time"
	"unsafe"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
)

type WindowCaptureStream unsafe.Pointer

type WindowCaptureRegion struct {
	OffsetX int
	OffsetY int
	Width   int
	Height  int

	DestWidth  int
	DestHeight int
}

type WindowH264Sample struct {
	Data []byte
	PTS  time.Duration
}

type WindowOpusSample struct {
	Data       []byte
	SampleRate int
	Channels   int
	Duration   time.Duration
	PTS        time.Duration
}

func StartWindowCaptureStreamWithCropAndDestSize(winTitle, appBundleID string, captureAudio bool, crop zgeo.Rect, destSize zgeo.Size) (WindowCaptureStream, error) {
	zlog.Info("StartWindowCaptureStreamWithCropAndDestSize:", winTitle, captureAudio, destSize)
	if !CanRecordScreen() {
		return nil, errors.New("screen recording permission is not granted")
	}

	C.EnsureCocoaGraphicsInitialized()

	cTitle := C.CString(winTitle)
	cApp := C.CString(appBundleID)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cApp))

	errBufLen := 1024
	errBuf := C.malloc(C.size_t(errBufLen))
	defer C.free(errBuf)
	*((*C.char)(errBuf)) = 0

	audio := C.int(0)
	if captureAudio {
		audio = 1
	}
	stream := C.startWindowCaptureStreamWithRegion(
		cTitle,
		cApp,
		audio,
		C.int(crop.Pos.X),
		C.int(crop.Pos.Y),
		C.int(crop.Size.W),
		C.int(crop.Size.H),
		C.int(destSize.W),
		C.int(destSize.H),
		(*C.char)(errBuf),
		C.int(errBufLen),
	)
	if stream == nil {
		errText := C.GoString((*C.char)(errBuf))
		if errText == "" {
			errText = "failed to start window capture stream"
		}
		zlog.Info("StartWindowCaptureStreamWithCropAndDestSize Err:", errText)
		return nil, errors.New(errText)
	}
	return WindowCaptureStream(stream), nil
}

// func StartWindowCaptureStreamWithRegion(winTitle, appBundleID string, captureAudio bool, region WindowCaptureRegion) (WindowCaptureStream, error) {
// 	crop := zgeo.RectFromXYWH(float64(region.OffsetX), float64(region.OffsetY), float64(region.Width), float64(region.Height))
// 	destSize := zgeo.SizeI(region.DestWidth, region.DestHeight)
// 	return StartWindowCaptureStreamWithCropAndDestSize(winTitle, appBundleID, captureAudio, crop, destSize)
// }

func StopWindowCaptureStream(stream WindowCaptureStream) {
	if stream == nil {
		return
	}
	C.stopWindowCaptureStream(unsafe.Pointer(stream))
}

func IsWindowCaptureStreamRunning(stream WindowCaptureStream) bool {
	if stream == nil {
		return false
	}
	return C.isWindowCaptureStreamRunning(unsafe.Pointer(stream)) != 0
}

func CaptureWindowStreamH264Sample(stream WindowCaptureStream) (WindowH264Sample, bool) {
	if stream == nil {
		return WindowH264Sample{}, false
	}
	var ptr *C.uchar
	var length C.int
	var ptsNS C.int64_t
	ok := C.copyLatestWindowStreamH264(unsafe.Pointer(stream), &ptr, &length, &ptsNS)
	if ok == 0 || ptr == nil || length <= 0 {
		return WindowH264Sample{}, false
	}
	defer C.free(unsafe.Pointer(ptr))
	data := C.GoBytes(unsafe.Pointer(ptr), length)
	return WindowH264Sample{Data: data, PTS: time.Duration(int64(ptsNS))}, true
}

func CaptureWindowStreamOpusSample(stream WindowCaptureStream) (WindowOpusSample, bool) {
	if stream == nil {
		return WindowOpusSample{}, false
	}
	var ptr *C.uchar
	var length C.int
	var sampleRate C.int
	var channels C.int
	var durationNS C.int64_t
	var ptsNS C.int64_t
	ok := C.copyLatestWindowStreamAudioOpus(unsafe.Pointer(stream), &ptr, &length, &sampleRate, &channels, &durationNS, &ptsNS)
	if ok == 0 || ptr == nil || length <= 0 || sampleRate <= 0 || channels <= 0 {
		return WindowOpusSample{}, false
	}
	defer C.free(unsafe.Pointer(ptr))
	data := C.GoBytes(unsafe.Pointer(ptr), length)
	return WindowOpusSample{
		Data:       data,
		SampleRate: int(sampleRate),
		Channels:   int(channels),
		Duration:   time.Duration(int64(durationNS)),
		PTS:        time.Duration(int64(ptsNS)),
	}, true
}
