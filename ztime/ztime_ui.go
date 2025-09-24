//go:build zui

package ztime

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zlocale"
)

func GetTimeWithServerLocation(t time.Time) time.Time {
	if !zlocale.IsDisplayServerTime.Get() {
		//		zlog.Info("GetTimeWithServerLocation", t, ServerTimezoneOffsetSecs, t.Location())
		t = t.Local()
		return t
	}
	name := fmt.Sprintf("UTC%+f", float64(ServerTimezoneOffsetSecs)/3600)
	loc := time.FixedZone(name, ServerTimezoneOffsetSecs)
	t = t.In(loc)
	return t
}
