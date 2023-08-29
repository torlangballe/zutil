package zbuild

import (
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

var (
	Build Info
)

type Info struct {
	At         time.Time
	User       string
	CommitHash string
	Branch     string
	Host       string
	Version    string
}

func SetFromLines(lines string) {
	zstr.RangeStringLines(lines, true, func(s string) bool {
		var name, val string
		if !zstr.SplitN(s, ":", &name, &val) {
			zlog.Error(nil, "bad build arg:", s)
			return true
		}
		switch name {
		case "BRANCH":
			Build.Branch = val
		case "HASH":
			Build.CommitHash = val
		case "AT":
			Build.At, _ = time.Parse(time.RFC3339, val)
		case "USER":
			Build.User = val
		case "HOST":
			Build.Host = val
		case "VERSION":
			Build.Version = val
		}
		return true
	})
}

func SetFromLine(lines string, linefeed, space string) {
	str := strings.Replace(lines, linefeed, "\n", -1)
	str = strings.Replace(str, space, " ", -1)
	SetFromLines(str)
}

func (info Info) ZUIString() string {
	str := zstr.Concat(" • ", info.Version, ztime.GetNice(info.At, true), info.CommitHash, info.Branch, info.User, info.Host)
	return str
}
