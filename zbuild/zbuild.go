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

func SetFromLine(line, sep, eq string) {
	for _, s := range strings.Split(line, sep) {
		var name, val string
		if !zstr.SplitN(s, ":", &name, &val) {
			zlog.Error("bad build arg:", s)
			return
		}
		switch name {
		case "BRANCH":
			Build.Branch = val
		case "HASH":
			Build.CommitHash = val
		case "AT":
			var err error
			Build.At, err = time.Parse(ztime.RFC3339NoZ, val)
			zlog.OnError(err, val)
		case "USER":
			Build.User = val
		case "HOST":
			Build.Host = val
		case "VERSION":
			Build.Version = val
		}
	}
}

func (info Info) ZUIString() string {
	str := zstr.Concat(" • ", info.Version, info.At.Format("15:04 02-Jan-07"), info.CommitHash, info.Branch, info.User, info.Host)
	return str
}
