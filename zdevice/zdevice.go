package zdevice

import (
	"errors"
	"net"
	"runtime"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/torlangballe/zutil/zlog"
)

// Interesting: https://github.com/jaypipes/ghw

type CellularNetworkType int
type OSType string

const (
	CellularUnknown CellularNetworkType = iota
	CellularWifiMax
	Cellular2G
	Cellular3G
	Cellular4G
	Cellular5G
	CellularXG

	MacOSType   OSType = "macos"
	WindowsType OSType = "windows"
	JSType      OSType = "js"
)

func OS() OSType {
	switch runtime.GOOS {
	case "windows":
		return WindowsType
	case "darwin":
		return MacOSType
	case "js":
		return JSType
	}
	zlog.Fatal(nil, "other type")
	return OSType("")
}

func OSVersion() string {
	info, err := host.Info()
	zlog.OnError(err)
	return info.PlatformVersion
}

// CPUUsage returns a slice of 0-1 where 1 is 100% of how much each CPU is utilized. Order unknown, but hopefully doesn't change
func CPUUsage() (out []float64) {
	coresVirtual, _ := cpu.Counts(true)
	coresPhysical, _ := cpu.Counts(false)

	threads := coresVirtual / coresPhysical
	percpu := true
	vals, err := cpu.Percent(0, percpu)
	if err != nil {
		zlog.Error(err)
		return
	}

	n := 0
	out = make([]float64, coresPhysical, coresPhysical)
	for i := 0; i < threads; i++ {
		for j := 0; j < coresPhysical; j++ {
			out[j] += float64(int(vals[n]) / threads)
			n++
		}
	}
	for j := 0; j < coresPhysical; j++ {
		out[j] /= 100
	}
	return
}

// GetMACAddress returns the MAC address as 6 bytes.
func MACAddress() ([]byte, error) {
	var mac net.HardwareAddr
	ifl, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, f := range ifl {
		if len(f.HardwareAddr) == 6 && f.Flags&(net.FlagLoopback|net.FlagPointToPoint) == 0 {
			mac = f.HardwareAddr
			break
		}
	}
	if len(mac) != 6 {
		return nil, errors.New("no suitable MAC address found")
	}
	return mac, nil
}
