package zgeo

import (
	"github.com/torlangballe/zutil/zdevice"
)

func init() {
	if zdevice.OS() == zdevice.MacOSType && zdevice.WasmBrowser() == "safari" {
		FontDefaultName = "-apple-system"
	}
}
