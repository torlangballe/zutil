// +build !js

package zdevice

import (
	"runtime"

	"github.com/denisbrodbeck/machineid"
	"github.com/shirou/gopsutil/disk"
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

func FreeAndUsedDiskSpace() (free int64, used int64) {
	parts, _ := disk.Partitions(true)
	for _, p := range parts {
		device := p.Mountpoint
		if runtime.GOOS == "darwin" && device != "/System/Volumes/Data" {
			continue
		}
		s, _ := disk.Usage(device)
		// zlog.Info("Free:", device, s.Free, "used:", s.Used)
		free += int64(s.Free)
		used += int64(s.Used)
	}
	zlog.Assert(used != 0)
	return
}
