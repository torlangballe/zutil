package zdevice

// #include <stdlib.h>
// #cgo LDFLAGS: -framework Foundation
// char *GetOSVersion();
import "C"
import (
	"time"

	"golang.org/x/sys/unix"
)

// func OSVersion() string {
// 	cstr := C.GetOSVersion()
// 	str := C.GoString(cstr)
// 	C.free(unsafe.Pointer(cstr))
// 	return str
// }

func BootTime() (time.Time, error) {
	epoc, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(int64(epoc.Sec), 0), nil
}
