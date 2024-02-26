//go:build zui

package zlocale

import (
	"github.com/torlangballe/zutil/zkeyvalue"
)

func init() {
	IsDisplayServerTime = zkeyvalue.NewOption[bool](nil, "ztime.IsDisplayServerTime", false)
	// zlog.Info("Change", IsDisplayServerTime.Key, IsDisplayServerTime.Get())
}
