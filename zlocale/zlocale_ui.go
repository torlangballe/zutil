//go:build zui

package zlocale

import (
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
)

func init() {
	IsDisplayServerTime = zkeyvalue.NewJSOption[bool]("ztime.IsDisplayServerTime", false)
	zlog.Info("Change IsDisplayServerTime", zlog.Pointer(IsDisplayServerTime))
}
