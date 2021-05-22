// +build !js

package zdevice

import (
	"github.com/denisbrodbeck/machineid"
	"github.com/torlangballe/zutil/zlog"
)

func WasmBrowser() string {
	return ""
}

// UUID returns a globally unique, permanent identifier string for the device we are running on.
// Returns a fixed, dummy id if running during tests.
func UUID() string {
	if zlog.IsInTests {
		return "01234567-89AB-CDEF-0123-456789ABCDEF"
	}
	str, err := machineid.ID()
	if err != nil {
		zlog.Fatal(err, "machineid.ID()")
	}
	return str
}

func OS() OSType {
	return Platform()
}
