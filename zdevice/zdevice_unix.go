//go:build !js && !windows

package zdevice

import (
	"errors"
	"runtime"
	"strconv"
	"strings"

	"github.com/matishsiao/goInfo"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
)

type NetCount struct {
	Bytes  int64
	Errors int64
	Drops  int64
}

type NetIO struct {
	In  NetCount
	Out NetCount
}

var (
	oldTotal         = map[string]NetIO{}
	currentBandwidth = map[string]NetIO{}
	sampled          bool
	bandwithInited   bool
)

const sampleSecs = 5

func InitNetworkBandwidth() {
	if bandwithInited {
		return
	}
	bandwithInited = true
	ztimer.RepeatForever(sampleSecs, func() {
		s, err := NetworkTraffic()
		if err != nil {
			zlog.Error("network", err)
			return
		}
		if sampled {
			m := map[string]NetIO{}
			for name, info := range s {
				var n NetIO
				old := oldTotal[name]
				n.In = info.In.Diff(old.In, sampleSecs)
				n.Out = info.Out.Diff(old.Out, sampleSecs)
				m[name] = n
			}
			currentBandwidth = m
		}
		sampled = true
		oldTotal = s
	})
}

func (n NetCount) Diff(old NetCount, div float64) NetCount {
	var diff NetCount
	diff.Bytes = int64(float64(n.Bytes-old.Bytes) / div)
	diff.Errors = int64(float64(n.Errors-old.Errors) / div)
	diff.Drops = int64(float64(n.Drops-old.Drops) / div)
	return diff
}

// func AllNetworkTraffic() (NetIO, error) {
// 	networks, err := net.IOCounters(false)
// 	if err != nil || len(networks) == 0 {
// 		return NetIO{}, err
// 	}
// 	n := statToNetIO(networks[0])
// 	return n, nil
// }

// NetworkBandwidthPerSec returns bytes pr sec/in out, and error/drop count pr sec all 0 if not sampled yet
func NetworkBandwidthPerSec() (map[string]NetIO, error) {
	if !sampled {
		return nil, errors.New("network bandwidth not sampled yet")
	}
	return currentBandwidth, nil
}

// NetworkTraffic returns a map of interface-name to In/Out bytes/drops/errors
func NetworkTraffic() (map[string]NetIO, error) {
	networks, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}
	m := map[string]NetIO{}
	for _, info := range networks {
		if info.BytesRecv == 0 && info.BytesSent == 0 {
			continue
		}
		m[info.Name] = statToNetIO(info)
	}
	return m, nil
}

func statToNetIO(info net.IOCountersStat) NetIO {
	var n NetIO
	n.In.Bytes = int64(info.BytesRecv)
	n.In.Errors = int64(info.Errin)
	n.In.Drops = int64(info.Dropin)
	n.Out.Bytes = int64(info.BytesSent)
	n.Out.Errors = int64(info.Errout)
	n.Out.Drops = int64(info.Dropout)
	return n
}

func HardwareTypeAndVersion() (string, float32) {
	str, err := zprocess.RunCommand("sysctl", 0, "-n", "hw.model")
	if err != nil {
		zlog.Error("run", err)
		return "", 0
	}
	i := strings.IndexAny(str, zstr.Digits)
	if i == -1 {
		return str, 1
	}
	name := zstr.Head(str, i)
	num, _ := strconv.ParseFloat(zstr.Body(str, i, -1), 32)

	return name, float32(num)
}

func Model() string {
	if runtime.GOOS == "darwin" {
		model, err := zprocess.RunCommand("sysctl", 0, "-n", "machdep.cpu.brand_string") // machdep.cpu.model
		if err != nil {
			zlog.Fatal("get model", err)
			return ""
		}
		return model
	}
	return "Server"
}

func OSVersion() string {
	gi, err := goInfo.GetInfo()
	if err != nil {
		zlog.Error("get info", err)
		return ""
	}
	return gi.Core
}

func FreeAndUsedDiskSpace() (free int64, used int64) {
	s, err := disk.Usage("/")
	if err != nil {
		return 0, 0
	}
	return int64(s.Free), int64(s.Used)
}

// func BootTime() (time.Time, error) {
// 	epoc, err := host.BootTime()
// 	if err != nil {
// 		return time.Time{}, err
// 	}
// 	return time.Unix(int64(epoc), 0), nil
// }
