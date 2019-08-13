package zlog

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/torlangballe/zutil/ustr"
)

type Priority int

const (
	Verbose Priority = iota
	DebugLevel
	InfoLevel
	WarningLevel
	ErrorLevel
	FatalLevel
)

var ErrorPriority = Verbose

var logcatMsgRegex = regexp.MustCompile(`([0-9]*)-([0-9]*)\s*([0-9]*):([0-9]*):([0-9]*).([0-9]*)\s*([0-9]*)\s*([0-9]*)\s*([VDIWEF])\s*(.*)`)

type LogCatItem struct {
	TimeStamp time.Time
	ProcessID int64
	ThreadID  int64
	Priority  Priority
	Rest      string // Everything after priority
	Tag       string // Tag : Message from Rest
	Message   string
}

func ParseLogcatMessage(s string) (log LogCatItem, got bool) {
	parts := logcatMsgRegex.FindStringSubmatch(s)
	if parts == nil {
		return
	}
	month, _ := strconv.Atoi(parts[1])
	day, _ := strconv.Atoi(parts[2])
	hour, _ := strconv.Atoi(parts[3])
	minute, _ := strconv.Atoi(parts[4])
	second, _ := strconv.Atoi(parts[5])
	microseconds, _ := strconv.Atoi(parts[6])
	log.ProcessID, _ = strconv.ParseInt(parts[7], 10, 64)
	log.ThreadID, _ = strconv.ParseInt(parts[8], 10, 64)
	log.Priority = Priority(parts[9][0])
	log.Rest = parts[10]

	log.TimeStamp = time.Date(time.Now().Year(), time.Month(month), day, hour, minute, second, microseconds*1e6, time.Local)
	if ustr.SplitN(log.Rest, ":", &log.Tag, &log.Message) {
		log.Tag = strings.TrimSpace(log.Tag)
		log.Message = strings.TrimSpace(log.Message)
	}
	got = true
	return
}

// Error performs Log with ErrorLevel priority
func Error(err error, parts ...interface{}) error {
	return log(err, ErrorLevel, 4, parts...)
}

// Error performs Log with ErrorLevel priority, getting stack from N
func ErrorAtStack(err error, stackPos int, parts ...interface{}) error {
	return log(err, ErrorLevel, stackPos, parts...)
}

// Log returns a new error combined with err (if not nil), and parts. Printing done if priority >= ErrorPriority
func Log(err error, priority Priority, parts ...interface{}) error {
	return log(err, priority, 4, parts...)
}

func log(err error, priority Priority, pos int, parts ...interface{}) error {
	finfo := GetCallingFunctionString(pos)
	p := strings.TrimSpace(fmt.Sprintln(parts...))
	if err != nil {
		err = errors.Wrap(err, p)
	} else {
		err = errors.New(p)
	}
	col := ustr.EscMagenta
	if priority >= ErrorLevel {
		col = ustr.EscRed
	}
	fmt.Println(finfo+": ", col+err.Error()+ustr.EscNoColor)
	if priority == FatalLevel {
		os.Exit(-1)
	}
	return err
}

func GetCallingFunctionInfo(pos int) (function, file string, line int) {
	pc, file, line, ok := runtime.Caller(pos)
	if ok {
		function = runtime.FuncForPC(pc).Name()
	}
	return
}

func GetCallingFunctionString(pos int) string {
	f, file, line := GetCallingFunctionInfo(pos)
	_, f = path.Split(f)
	_, file = path.Split(file)
	return fmt.Sprintf("%s%s()%s @ %s%s:%d%s", ustr.EscCyan, f, ustr.EscNoColor, ustr.EscGreen, file, line, ustr.EscNoColor)
}
