//go:build !js && !windows
// +build !js,!windows

package zdevice

import (
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"golang.org/x/sys/unix"
)

func HardwareTypeAndVersion() (string, float32) {
	str, err := zprocess.RunCommand("sysctl", 0, "-n", "hw.model")
	if err != nil {
		zlog.Error(err)
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
	model, _ := unix.SysctlUint32("machdep.cpu.model")
	return strconv.FormatUint(uint64(model), 10)
}
