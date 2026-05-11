package zdesktop

import (
	"image"

	"github.com/torlangballe/zutil/zgeo"
)

type CameraStream *struct{}

func StartCameraCaptureStream(cs CameraStream) CameraStream {
	return nil
}

func StopCameraCaptureStream(cs CameraStream) {
}

func IsCameraCaptureStreamRunning(cs CameraStream) bool {
	return false
}

func CaptureCameraStreamImage(cs CameraStream, cropRect zgeo.Rect) (image.Image, error) {
	return nil, nil
}

func CanUseCamera() bool {
	return false
}
