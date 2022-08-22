//go:build !js && !windows
// +build !js,!windows

package zdevice

import (
	"os"
	"strconv"
	"strings"

	"github.com/matishsiao/goInfo"
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
	model, err := zprocess.RunCommand("sysctl", 0, "-n", "machdep.cpu.model")
	if err != nil {
		zlog.Fatal(err, "get model")
		return ""
	}
	return model
}

func OSVersion() string {
	gi := goInfo.GetInfo()
	return gi.Core
}

func FreeAndUsedDiskSpace() (free int64, used int64) {
	var stat unix.Statfs_t
	wd, _ := os.Getwd()
	unix.Statfs(wd, &stat)
	free = int64(stat.Bfree * uint64(stat.Bsize))
	used = int64(stat.Blocks * uint64(stat.Bsize))
	zlog.Assert(used != 0)
	return
}
