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

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zstr"
)

type Priority int

type StackAdjust int

const (
	Verbose Priority = iota
	DebugLevel
	InfoLevel
	WarningLevel
	ErrorLevel
	FatalLevel
)

var ErrorPriority = Verbose
var OutputFilePath = ""
var logcatMsgRegex = regexp.MustCompile(`([0-9]*)-([0-9]*)\s*([0-9]*):([0-9]*):([0-9]*).([0-9]*)\s*([0-9]*)\s*([0-9]*)\s*([VDIWEF])\s*(.*)`)
var outFile *os.File
var UseColor = false

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
	if zstr.SplitN(log.Rest, ":", &log.Tag, &log.Message) {
		log.Tag = strings.TrimSpace(log.Tag)
		log.Message = strings.TrimSpace(log.Message)
	}
	got = true
	return
}

// Error performs Log with ErrorLevel priority
func Error(err error, parts ...interface{}) error {
	return baseLog(err, ErrorLevel, 4, parts...)
}

// Fatal performs Log with Fatal priority
func Fatal(err error, parts ...interface{}) error {
	return baseLog(err, FatalLevel, 4, parts...)
}

// Info performs Log with InfoLevel priority
func Info(parts ...interface{}) {
	baseLog(nil, InfoLevel, 4, parts...)
}

func Dummy(parts ...interface{}) {
}

// Debug performs Log with DebugLevel priority
func Debug(parts ...interface{}) {
	baseLog(nil, DebugLevel, 4, parts...)
}

// Error performs Log with ErrorLevel priority, getting stack from N
func ErrorAtStack(err error, stackPos int, parts ...interface{}) error {
	return baseLog(err, ErrorLevel, stackPos, parts...)
}

// Log returns a new error combined with err (if not nil), and parts. Printing done if priority >= ErrorPriority
func Log(err error, priority Priority, parts ...interface{}) error {
	return baseLog(err, priority, 4, parts...)
}

func baseLog(err error, priority Priority, pos int, parts ...interface{}) error {
	if len(parts) != 0 {
		n, got := parts[0].(StackAdjust)
		if got {
			parts = parts[1:]
			pos += int(n)
		}
	}
	col := ""
	endCol := ""
	if UseColor {
		col = zstr.EscYellow
		if priority >= ErrorLevel {
			col = zstr.EscMagenta
		}
		endCol = zstr.EscNoColor
	}
	finfo := ""
	if priority != InfoLevel {
		finfo = GetCallingFunctionString(pos) + ": "
	}
	p := strings.TrimSpace(fmt.Sprintln(parts...))
	if err != nil {
		err = errors.Wrap(err, p)
	} else {
		err = errors.New(p)
	}
	fmt.Println(finfo + col + err.Error() + endCol)

	if OutputFilePath != "" && outFile == nil {
		var ferr error
		fp := zfile.ExpandTildeInFilepath(OutputFilePath)
		outFile, ferr = os.Create(fp)
		if ferr != nil {
			fmt.Println("Error creating output file for zlog:", ferr)
			OutputFilePath = ""
		}
	}
	str := finfo + err.Error()
	if outFile != nil {
		outFile.WriteString(str + "\n")
	}
	writeToSyslog(str)
	if priority == FatalLevel {
		panic("zlog.Fatal")
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

func GetCallingStackString() string {
	var parts []string
	for i := 3; ; i++ {
		s := GetCallingFunctionString(i)
		if s == "" {
			break
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n")
}

func GetCallingFunctionString(pos int) string {
	f, file, line := GetCallingFunctionInfo(pos)
	if f == "" {
		return ""
	}
	_, f = path.Split(f)
	_, file = path.Split(file)
	return fmt.Sprintf("%s() @ %s:%d", f, file, line)
}

func Assert(success bool, parts ...interface{}) {
	if !success {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Fatal(errors.New("assert failed"), parts...)
	}
}

func AssertMakeError(success bool, parts ...interface{}) error {
	if !success {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		return Error(errors.New("assert failed"), parts...)
	}
	return nil
}

func ErrorIf(check bool, parts ...interface{}) bool {
	if check {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Error(errors.New("error if occured:"), parts...)
	}
	return check
}

func OnError(err error, parts ...interface{}) {
	if err != nil {
		Error(err, parts...)
	}
}

func AssertNotErr(err error, parts ...interface{}) {
	if err != nil {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Fatal(err, parts...)
	}
}
