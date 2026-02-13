//go:build server

package zrequests

import (
	"sort"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/ztermfields"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

const RedirMethod = "REDIR"

type LogID struct {
	Method    string
	Secondary string
	URL       string `zui:"cols:110,diff"`
}

type LogEntry struct {
	Count       int `zui:"justify:right"`
	LastUse     time.Time
	MinDuration time.Duration
	MaxDuration time.Duration
}

type LogRow struct {
	LogEntry
	LogID
}

type Logger struct {
	logs        zmap.LockMap[LogID, LogEntry]
	deleteTimer *ztimer.Repeater
}

type Commands struct {
	ztermfields.SliceCommander
}

var (
	CommandsNode Commands
	MainLogger   = &Logger{}
)

func init() {
	CommandsNode.SlicePointerFunc = func(c *zcommands.CommandInfo) any {
		count := MainLogger.logs.Count()
		list := make([]LogRow, count, count)
		i := 0
		MainLogger.logs.ForEach(func(id LogID, e LogEntry) bool {
			var r LogRow
			r.LogID = id
			r.LogEntry = e
			list[i] = r
			i++
			return true
		})
		sort.Slice(list, func(i, j int) bool {
			li := list[i]
			lj := list[j]
			if li.Count != lj.Count {
				return li.Count > lj.Count
			}
			c := strings.Compare(li.URL, lj.URL)
			if c != 0 {
				return c > 0
			}
			return li.MaxDuration > lj.MaxDuration
		})
		if len(list) > 300 {
			list = list[:300]
		}
		return &list
	}
}

func NewLogger() *Logger {
	l := &Logger{}
	l.deleteTimer = ztimer.RepeatForever(60*60, func() {
		l.logs.ForEach(func(id LogID, e LogEntry) bool {
			since := time.Since(e.LastUse)
			if e.Count == 1 && since > ztime.Day || e.Count > 1 && since > ztime.Day*7 {
				l.logs.Remove(id)
			}
			return true
		})
	})
	return l
}

func (l *Logger) Add(surl, secondary, method string, in bool, dur time.Duration) {
	// zlog.Info("zeq.Add:", surl)
	if method != RedirMethod {
		if in {
			method += ":IN"
		} else {
			method += ":OUT"
		}
	}
	id := LogID{
		URL:       surl,
		Secondary: secondary,
		Method:    method,
	}
	e, got := l.logs.Get(id)
	if !got {
		e.MinDuration = dur
	} else if e.MinDuration > dur {
		e.MinDuration = dur
	}
	if e.MaxDuration < dur {
		e.MaxDuration = dur
	}
	e.Count++
	e.LastUse = time.Now()
	l.logs.Set(id, e)
}
