//go:build !js

package zdevice

import (
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/torlangballe/zutil/zlog"
)

func WasmBrowser() string {
	return ""
}

// UUID returns a globally unique, permanent identifier string for the device we are running on.
// Returns a fixed, dummy id if running during tests.
// Format will always be in UUID 8-4-4-4-12 hex chars.
func UUID() string {
	if zlog.IsInTests {
		return "01234567-89AB-CDEF-0123-456789ABCDEF"
	}
	str, err := machineid.ID()
	if err != nil {
		zlog.Fatal(err, "machineid.ID()")
	}
	str = strings.ToUpper(str)
	if len(str) == 32 {
		str = str[:8] + "-" + str[8:12] + "-" + str[12:16] + "-" + str[16:20] + "-" + str[20:]
	}
	return str
}

func OS() OSType {
	return Platform()
}
