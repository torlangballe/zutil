package zdevice

import (
	"syscall/js"
	"time"

	ua "github.com/mileusna/useragent"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zstr"
)

// https://developer.mozilla.org/en-US/docs/Web/API/Navigator - info about browser/dev

var userAgent *ua.UserAgent

func init() {
	if OS() == MacOSType && WasmBrowser() == "safari" {
		zgeo.SetSafariDefaultFont()
	}
}

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

func WasmBrowser() BrowserType {
	switch getUserAgentInfo().Name {
	case ua.Chrome:
		return Chrome
	case ua.Safari:
		return Safari
	case ua.Edge:
		return Edge
	case ua.Firefox:
		return Firefox
	}
	return BrowserNone
}

// OS Returns the underlying operating system program is running on.
// For wasm in browsers, this is the actual mac/win/linux etc os the browser is running on.
func OS() OSType {
	u := getUserAgentInfo()
	return OSTypeFromUserAgent(u)
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

func OSVersion() string {
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

func UUID() string {
	const webHardwareIDKey = "zutil.zdevice.WebHardwareIDKey"
	id, _ := zkeyvalue.DefaultStore.GetString(webHardwareIDKey)
	if id == "" {
		id = zstr.GenerateUUID()
		zkeyvalue.DefaultStore.SetString(id, webHardwareIDKey, true)
	}
	return id
}
