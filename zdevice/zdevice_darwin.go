package zdevice

// #include <stdlib.h>
// #cgo LDFLAGS: -framework Foundation
// char *GetOSVersion();
import "C"
import (
	"unsafe"
)

func OSVersion() string {
	cstr := C.GetOSVersion()
	str := C.GoString(cstr)
	C.free(unsafe.Pointer(cstr))
	return str
}
