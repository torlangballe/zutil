package zdevice

import (
	"syscall/js"
	"time"

	ua "github.com/mileusna/useragent"
	"github.com/torlangballe/zutil/zlog"
)

// https://developer.mozilla.org/en-US/docs/Web/API/Navigator - info about browser/dev

var userAgent *ua.UserAgent

func getUserAgentInfo() *ua.UserAgent {
	if userAgent == nil {
		ustr := js.Global().Get("navigator").Get("userAgent").String()
		u := ua.Parse(ustr)
		userAgent = &u
	}
	return userAgent
}

func IsIPad() bool {
	return false
}

func IsBrowser() bool {
	return true
}

func WasmBrowser() string {
	return getUserAgentInfo().Name
}

// OS Returns the underlying operating system program is running on.
// For wasm in browsers, this is the actual mac/win/linux etc os the browser is running on.
func OS() OSType {
	switch getUserAgentInfo().OS {
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
	zlog.Error(nil, "other type")
	return OSType("")
}

func IsIPhone() bool {
	return false
}

func Name() string {
	return ""
}

func FingerPrint() string {
	return ""
}

func IdentifierForVendor() string {
	return ""
}

func Manufacturer() string {
	return ""
}

func BatteryLevel() float32 {
	return 0
}

func IsCharging() int {
	return 0
}

func OSVersionstring() string {
	return ""
}

func TimeZone() *time.Location {
	return time.UTC
}

func Type() string {
	return ""
}

func HardwareModel() string {
	return ""
}

func HardwareBrand() string {
	return ""
}

func OSPlatform() string {
	return ""
}

func FreeAndUsedDiskSpace() (int64, int64) {
	return 0, 0
}

func IsWifiEnabled() bool {
	return false
}

func WifiIPv4Address() string {
	return ""
}

func Ipv4Address() string {
	return ""
}

func IPv6Address() string {
	return ""
}

func GetMainMACint64() string {
	return ""
}

func LanMACint64() string {
	return ""
}

func WifiMint64() string {
	return ""
}

func WifiLinkSpeed() string {
	return ""
}

func CellularNetwork() CellularNetworkType {
	return CellularUnknown
}

func HardwareTypeAndVersion() (string, float32) {
	return getUserAgentInfo().Device, 1
}
