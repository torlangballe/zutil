package zdevice

// #include <stdlib.h>
// #cgo LDFLAGS: -framework Foundation
// char *GetOSVersion();
import "C"

// func OSVersion() string {
// 	cstr := C.GetOSVersion()
// 	str := C.GoString(cstr)
// 	C.free(unsafe.Pointer(cstr))
// 	return str
// }
