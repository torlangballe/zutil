// +build !js

package zdevice

import (
	"github.com/denisbrodbeck/machineid"
	"github.com/torlangballe/zutil/zlog"
)

func WasmBrowser() string {
	return ""
}

func UUID() string {
	str, err := machineid.ID()
	if err != nil {
		zlog.Fatal(err, "machineid.ID()")
	}
	return str
}

func OS() OSType {
	return Platform()
}
