package zlog

import (
	"fmt"
	"net/http/pprof"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
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

var (
	PrintPriority = Verbose
	outputHooks   = map[string]func(s string){}
	UseColor      = false
	PanicHandler  func(reason string, exit bool)
	IsInTests     bool
)

func init() {
	IsInTests = (strings.HasSuffix(os.Args[0], ".test"))
	PanicHandler = func(reason string, exit bool) {
		Error(nil, "panic handler:", reason)
		if exit {
			panic(reason)
		}
	}
}

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

// Log returns a new error combined with err (if not nil), and parts. Printing done if priority >= PrintPriority
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
	if priority < ErrorLevel && IsInTests {
		return nil
	}
	for i, p := range parts {
		n, got := p.(StackAdjust)
		if got {
			parts = append(parts[:i], parts[i+1:]...)
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

func makePathRelativeTo(path, rel string) string {
	origPath := path
	path = strings.TrimLeft(path, "/")
	rel = strings.TrimLeft(rel, "/")
	// fmt.Println("MakePathRelativeTo1:", path, rel)
	for {
		p := zstr.HeadUntil(path, "/")
		r := zstr.HeadUntil(rel, "/")
		if p != r || p == "" {
			break
		}
		l := len(p)
		path = zstr.Body(path, l+1, -1)
		rel = zstr.Body(rel, l+1, -1)
	}
	// fmt.Println("MakePathRelativeTo:", path, rel)
	count := strings.Count(rel, "/")
	if count != 0 {
		count++
	}
	str := strings.Repeat("../", count) + path
	if count > 2 || len(str) > len(origPath) {
		var rest string
		if runtime.GOOS == "js" {
			return origPath
		}
		usr, err := user.Current()
		if err != nil {
			return origPath
		}
		dir := usr.HomeDir + "/"
		if zstr.HasPrefix(origPath, dir, &rest) {
			return "~/" + rest
		}
		return origPath
	}
	return str
}

func GetCallingFunctionString(pos int) string {
	function, file, line := GetCallingFunctionInfo(pos)
	if function == "" {
		return ""
	}
	_, function = path.Split(function)

	home, _ := os.Getwd()
	file = makePathRelativeTo(file, home)

	return fmt.Sprintf("%s:%d %s()", file, line, function)
}

func Assert(success bool, parts ...interface{}) {
	if !success {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		fmt.Println("ASSERT", parts)
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

func HandlePanic(exit bool) error {
	if PanicHandler == nil {
		return nil
	}
	if runtime.GOOS == "js" {
		return nil
	}
	r := recover()
	if r != nil {
		Error(nil, "\n🟥HandlePanic:", r, "\n", GetCallingStackString())
		str := fmt.Sprint(r)
		PanicHandler(str, exit)
		e, _ := r.(error)
		if e != nil {
			return e
		}
		return errors.New(str)
	}
	return nil
}

type Profile struct {
	Name  string
	Start time.Time
	Lines []Line
}

type Line struct {
	Text     string
	Duration time.Duration
	Time     time.Time
}

var profiles []Profile

func PushProfile(name string) {
	p := Profile{}
	p.Name = name
	p.Start = time.Now()
	profiles = append(profiles, p)
}

func ProfileLog(parts ...interface{}) {
	var line *Line
	previ := -1
	p := &profiles[len(profiles)-1]
	str := zstr.SprintSpaced(parts...)
	for i, l := range p.Lines {
		if l.Text == str {
			line = &p.Lines[i]
			previ = i - 1
			break
		}
	}
	if line == nil {
		previ = len(p.Lines) - 1
		p.Lines = append(p.Lines, Line{})
		line = &p.Lines[previ+1]
		line.Time = time.Now()
		line.Text = str
	}
	start := p.Start
	if previ > -1 {
		start = p.Lines[previ].Time
	}
	line.Duration += time.Since(start)
	// Info("Add:", p.Name, line.Duration)
}

func EndProfile(parts ...interface{}) {
	if len(parts) != 0 {
		ProfileLog(parts...)
	}
	p := &profiles[len(profiles)-1]
	dur := time.Since(p.Start)
	for _, l := range p.Lines {
		percent := int(float64(l.Duration) / float64(dur) * 100)
		Info(p.Name+":", l.Text, time.Since(p.Start), "    ", l.Duration, percent, "%")
	}
	RemoveProfile()
}

func RemoveProfile() {
	profiles = profiles[:len(profiles)-1]
}

func OnErrorTestError(t *testing.T, err error, items ...interface{}) bool {
	if err != nil {
		items = append([]interface{}{err}, items...)
		t.Error(items...)
		return true
	}
	return false
}
