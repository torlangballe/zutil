//go:build !js
// +build !js

package zdevice

import (
	"os"

	"github.com/denisbrodbeck/machineid"
	"github.com/torlangballe/zutil/zlog"
	"golang.org/x/sys/unix"
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

func FreeAndUsedDiskSpace() (free int64, used int64) {
	var stat unix.Statfs_t
	wd, _ := os.Getwd()
	unix.Statfs(wd, &stat)
	free = int64(stat.Bfree * uint64(stat.Bsize))
	used = int64(stat.Blocks * uint64(stat.Bsize))
	zlog.Assert(used != 0)
	return
}
