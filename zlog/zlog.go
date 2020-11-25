package zlog

import (
	"context"
	"fmt"
	"net/http/pprof"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
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

var (
	ErrorPriority  = Verbose
	OutputFilePath = ""
	outputHooks    = map[string]func(s string){}
	outFile        *os.File
	UseColor       = false
)

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

// Info performs Log with InfoLevel priority
func Warn(parts ...interface{}) {
	baseLog(nil, WarningLevel, 4, parts...)
}

// func Dummy(parts ...interface{}) {
// }

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

func expandTildeInFilepath(path string) string {
	if runtime.GOOS == "js" {
		return ""
	}
	usr, err := user.Current()
	if err == nil {
		dir := usr.HomeDir
		return strings.Replace(path, "~", dir, 1)
	}
	return ""
}

var hookingLock sync.Mutex
var hooking = false
var timeLock sync.Mutex
var datePrinted time.Time
var linesPrintedSinceTimeStamp int

func NewError(parts ...interface{}) error {
	var err error
	if len(parts) > 0 {
		err, _ = parts[0].(error)
		if err != nil {
			parts = parts[1:]
		}
	}
	p := strings.TrimSpace(fmt.Sprintln(parts...))
	pnew := zstr.ColorSetter.Replace(p)
	if pnew != p {
		p = pnew + zstr.EscNoColor
	}
	if err != nil {
		err = errors.Wrap(err, p)
	} else {
		err = errors.New(p)
	}
	return err
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
		if priority >= ErrorLevel {
			col = zstr.EscMagenta
			endCol = zstr.EscNoColor
		} else if priority >= WarningLevel {
			col = zstr.EscYellow
			endCol = zstr.EscNoColor
		}
	}
	finfo := ""
	if runtime.GOOS != "js" {
		timeLock.Lock()
		linesPrintedSinceTimeStamp++
		if time.Since(datePrinted) > time.Second*60 || linesPrintedSinceTimeStamp > 10 {
			finfo = fmt.Sprintln("goroutines:", runtime.NumGoroutine())
			datePrinted = time.Now()
		}
		finfo += zstr.EscCyan + time.Now().Local().Format("15:04:05/02 ") + zstr.EscNoColor
		linesPrintedSinceTimeStamp = 0
		timeLock.Unlock()
	}
	if priority != InfoLevel {
		finfo += GetCallingFunctionString(pos) + ": "
	}
	if err != nil {
		parts = append([]interface{}{err}, parts...)
	}
	if priority == FatalLevel {
		finfo += "\nFatal:" + GetCallingStackString() + "\n"
	}
	err = NewError(parts...)
	fmt.Println(finfo + col + err.Error() + endCol)
	str := finfo + err.Error() + "\n"
	WriteToTheLogFile(str)

	hookingLock.Lock()
	if !hooking {
		hooking = true
		for _, f := range outputHooks {
			f(str)
		}
		hooking = false
	}
	hookingLock.Unlock()
	writeToSyslog(str)
	if priority == FatalLevel {
		os.Exit(-1)
	}
	return err
}

func WriteToTheLogFile(str string) {
	if OutputFilePath != "" && outFile == nil {
		var ferr error
		fp := expandTildeInFilepath(OutputFilePath)
		outFile, ferr = os.Create(fp)
		if ferr != nil {
			fmt.Println("Error creating output file for zlog:", ferr)
			OutputFilePath = ""
		}
	}
	if outFile != nil {
		outFile.WriteString(str)
	}
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
	function, file, line := GetCallingFunctionInfo(pos)
	if function == "" {
		return ""
	}
	_, function = path.Split(function)

	home, _ := os.Getwd()
	file = zfile.MakePathRelativeTo(file, home)

	return fmt.Sprintf("%s:%d %s()", file, line, function)
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

func OnError(err error, parts ...interface{}) bool {
	if err != nil {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Error(err, parts...)
		return true
	}
	return false
}

func AssertNotError(err error, parts ...interface{}) {
	if err != nil {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Fatal(err, parts...)
	}
}

func AddHook(id string, call func(s string)) {
	outputHooks[id] = call
}

type WrappedError struct {
	Text  string
	Error error
}

func (w *WrappedError) Unwrap() error {
	return w.Error
}

func Wrap(err error, parts ...interface{}) error {
	p := strings.TrimSpace(fmt.Sprintln(parts...))
	return errors.Wrap(err, p)

}

func PrintStartupInfo(version, commitHash, builtAt, builtBy, builtOn string) {
	_, name := filepath.Split(os.Args[0])
	Info("\n"+zstr.EscYellow+"START:",
		name,
		"v"+version,
		"Build:",
		commitHash,
		builtAt,
		builtBy,
		builtOn,
		zstr.EscNoColor,
	)
}

func SetProfilingHandle(r *mux.Router) {
	r.HandleFunc("/debug/pprof/", pprof.Index)
	//	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	// r.HandleFunc("/debug/pprof/profile", pprof.Index)
	// r.HandleFunc("/debug/pprof/heap", pprof.Index)
	//	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	//	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

func RunProcessUntilTimeouSecs(secs float64, do func()) (completed bool) {
	ctx, _ := context.WithTimeout(context.Background(), ztime.SecondsDur(secs))
	return RunProcessUntilContextTimeout(ctx, do)
}

func RunProcessUntilContextTimeout(ctx context.Context, do func()) (completed bool) {
	doneChannel := make(chan struct{}, 2)
	go func() {
		do()
		doneChannel <- struct{}{}
	}()
	select {
	case <-doneChannel:
		return true
	case <-ctx.Done():
		return false
	}
}
