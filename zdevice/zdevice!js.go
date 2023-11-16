//go:build !js

package zdevice

import (
	"os"
	"strings"

	"github.com/denisbrodbeck/machineid"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
)

func WasmBrowser() BrowserType {
	return BrowserNone
}

// UUID returns a globally unique, permanent identifier string for the device we are running on.
// Returns a fixed, dummy id if running during tests.
// Format will always be in UUID 8-4-4-4-12 hex chars.
func UUID() string {
	if zdebug.IsInTests {
		return TestDeviceUUID
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

func Name() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return name
}
