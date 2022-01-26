package zdevice

import (
	"errors"
	"net"
	"runtime"

	// "github.com/shirou/gopsutil/cpu"

	//	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/mem"
	"github.com/torlangballe/zutil/zlog"
)

// Interesting: https://github.com/jaypipes/ghw

type CellularNetworkType int
type OSType string
type ArchitectureType string

const (
	CellularUnknown CellularNetworkType = iota
	CellularWifiMax
	Cellular2G
	Cellular3G
	Cellular4G
	Cellular5G
	CellularXG

	MacOSType   OSType = "macos"
	IOSType     OSType = "ios"
	AndroidType OSType = "android"
	WindowsType OSType = "windows"
	LinuxType   OSType = "linux"
	JSType      OSType = "js"

	ARM64Type            ArchitectureType = "arm64"
	AMD64Type            ArchitectureType = "amd64"
	WASMType             ArchitectureType = "wasm"
	ArchitectureTypeNone ArchitectureType = ""
)

// Platform is the surface-system we are running on.
// For wasm in browser this is JSType, not underlying os browser is running on
func Platform() OSType {
	switch runtime.GOOS {
	case "windows":
		return WindowsType
	case "darwin":
		return MacOSType
	case "js":
		return JSType
	case "android":
		return AndroidType
	}
	zlog.Fatal(nil, "other type")
	return OSType("")
}

func IsDesktop() bool {
	return OS() != AndroidType && OS() != IOSType
}

func Model() string {
	infos, err := cpu.Info()
	if err != nil || len(infos) < 1 {
		zlog.Error(err)
		return ""
	}
	return infos[0].Model
}

// Architecture returns the main type of CPU used, ARM, AMD64, WASM
func Architecture() ArchitectureType {
	if runtime.GOARCH == "arm64" {
		return ARM64Type
	}
	if runtime.GOARCH == "amd64" {
		return AMD64Type
	}
	if runtime.GOARCH == "wasm" {
		return WASMType
	}
	return ArchitectureTypeNone
}

// CPUUsage returns a slice of 0-1 where 1 is 100% of how much each CPU is utilized. Order unknown, but hopefully doesn't change
// if more than maxCores, it is recursivly halved, summing first half with last
func CPUUsage(maxCores int) (out []float64) {
	if zlog.IsInTests {
		return []float64{0.1, 0.2, 0.3, 0.4}
	}
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
	for len(out) > maxCores {
		if len(out)%2 == 1 {
			out = out[:maxCores]
			return
		}
		half := len(out) / 2
		for i := 0; i < half; i++ {
			out[i] = (out[i] + out[half+i]) / 2
		}
		out = out[:half]
	}
	zlog.Info("CPUUsage:", out, maxCores)
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

func MemoryAvailableUsedAndTotal() (available uint64, used uint64, total uint64) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		zlog.Fatal(err, "get vm")
	}
	return vm.Available, vm.Used, vm.Total
}
