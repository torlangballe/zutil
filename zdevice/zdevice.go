package zdevice

import (
	"errors"
	"net"
	"runtime"
	"strconv"
	"strings"
	"time"

	ua "github.com/mileusna/useragent"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
)

// Interesting: https://github.com/jaypipes/ghw

type CellularNetworkType int
type OSType string
type ArchitectureType string
type BrowserType string

const (
	Safari         BrowserType = "safari"
	Chrome         BrowserType = "chrome"
	Edge           BrowserType = "edge"
	Firefox        BrowserType = "firefox"
	HeadlessChrome BrowserType = "headless-chrome"
	Default        BrowserType = "default"
	BrowserNone    BrowserType = ""
)

var (
	OverrideUUID string // used for unit tests and other testing that needs a know uuid
	lastCPUGet   time.Time
	lastCPU      []float64
)

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
	WebType     OSType = "web"
	NoneOSType  OSType = ""

	ARM64Type            ArchitectureType = "arm64"
	AMD64Type            ArchitectureType = "amd64"
	WASMType             ArchitectureType = "wasm"
	ArchitectureTypeNone ArchitectureType = ""
)

func init() {
	zdebug.AverageCPUFunc = func() float64 {
		return zfloat.Average(CPUUsage(6))
	}
}

// Platform is the surface-system we are running on.
// For wasm in browser this is WebType, not underlying os browser is running on
func Platform() OSType {
	switch runtime.GOOS {
	case "windows":
		return WindowsType
	case "darwin":
		return MacOSType
	case "js":
		return WebType
	case "android":
		return AndroidType
	case "linux":
		return LinuxType
	}
	zlog.Fatal("other type")
	return OSType("")
}

func IsDesktop() bool {
	os := OS()
	return os != AndroidType && os != IOSType
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

func CPUAverage() float64 {
	all := CPUUsage(1)
	if len(all) == 0 {
		return -1
	}
	return all[0]
}

// CPUUsage returns a slice of 0-1 where 1 is 100% of how much each CPU is utilized. Order unknown, but hopefully doesn't change
// if more than maxCores, it is recursivly halved, summing first half with last
func CPUUsage(maxCores int) (out []float64) {
	if runtime.GOOS == "js" {
		return []float64{-1}
	}
	if zdebug.IsInTests {
		return []float64{0.1, 0.2, 0.3, 0.4}
	}
	coresVirtual, _ := cpu.Counts(true)
	coresPhysical, _ := cpu.Counts(false)

	threads := coresVirtual / coresPhysical
	percpu := true
	var vals []float64
	if time.Since(lastCPUGet) < time.Second { // this isn't just for efficiency, cpu.Percent() twice immediately returns all 0's.
		vals = lastCPU
	} else {
		lastCPUGet = time.Now()
		var err error
		vals, err = cpu.Percent(0, percpu)
		if err != nil {
			zlog.Error("cpu.percent", err)
			return
		}
		lastCPU = vals
	}
	n := 0
	out = make([]float64, coresPhysical)
	for i := 0; i < threads; i++ {
		for j := 0; j < coresPhysical; j++ {
			out[j] += vals[n] / float64(threads)
			n++
		}
	}
	for len(out) > maxCores {
		half := len(out) / 2
		out = out[:half*2]
		for i := 0; i < half; i++ {
			out[i] = (out[i] + out[half+i]) / 2
		}
		out = out[:half]
	}
	for i := range out {
		out[i] /= 100
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

func MemoryAvailableUsedAndTotal() (available int64, used int64, total int64) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		zlog.Fatal("get vm", err)
	}
	return int64(vm.Available), int64(vm.Used), int64(vm.Total)
}

func OSTypeFromUserAgentString(uas string) OSType {
	u := ua.Parse(uas)
	return OSTypeFromUserAgent(&u)
}

func OSTypeFromUserAgent(u *ua.UserAgent) OSType {
	switch u.OS {
	case ua.MacOS:
		return MacOSType
	case ua.Windows:
		return WindowsType
	case ua.Linux:
		return LinuxType
	case ua.IOS:
		return IOSType
	case ua.Android:
		return AndroidType
	}
	zlog.Error("other type")
	return OSType("")
}

func OSVersionNumber() float64 {
	str := OSVersion()
	var snum string
	parts := strings.SplitN(str, ".", 3)
	switch len(parts) {
	case 0:
		zlog.Error("no parts", str)
		return 0
	case 1:
		snum = parts[0]
	default:
		snum = parts[0] + "." + parts[1]
	}
	n, err := strconv.ParseFloat(snum, 64)
	zlog.OnError(err, str, snum)
	return n
}

func IsRunningOnDebugMachine() bool {
	htype, _ := HardwareTypeAndVersion()
	return (htype == "MacBookPro")
}
