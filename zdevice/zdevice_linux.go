package zdevice

import (
	"strconv"
	"strings"
	"time"

	"github.com/matishsiao/goInfo"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

func BootTime() (t time.Time, err error) {
	var secs float64
	zfile.ForAllFileLines("/proc/uptime", true, func(str string) bool {
		f := strings.Fields(str)
		secs, err = strconv.ParseFloat(f[0], 64)
		return true
	})
	if err != nil {
		return t, err
	}
	t = time.Now().Add(ztime.SecondsDur(secs))
	return t, nil
}

func OSVersion() string {
	gi, err := goInfo.GetInfo()
	if err != nil {
		zlog.Error("get info", err)
		return ""
	}
	return gi.Core
}
