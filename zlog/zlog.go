package zlog

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
)

type Priority int
type StackAdjust int
type LimitID string
type Enabler bool

const (
	VerboseLevel Priority = iota
	DebugLevel
	InfoLevel
	WarningLevel
	ErrorLevel
	FatalLevel
)

var (
	PrintPriority           = DebugLevel
	outputHooks             = map[string]func(s string){}
	UseColor                = false
	PanicHandler            func(reason string, exit bool)
	PrintGoRoutines         = false
	PrintDate               = true
	lastGoRoutineCount      int
	lastGoRoutineOutputTime time.Time
	rateLimiters            zmap.LockMap[LimitID, time.Time]
	EnablerList             zmap.LockMap[string, *Enabler]

	isInTests = (strings.HasSuffix(os.Args[0], ".test"))
)

func init() {
	PanicHandler = func(reason string, exit bool) {
		Error("panic handler:", reason)
		if exit {
			panic(reason)
		}
	}
}

func Error(parts ...interface{}) error {
	return baseLog(ErrorLevel, 4, parts...)
}

// Fatal performs Log with Fatal priority
func Fatal(parts ...interface{}) error {
	return baseLog(FatalLevel, 4, parts...)
}

func FatalNotImplemented() {
	Fatal(nil, "Not Implemented")
}

// Info performs Log with InfoLevel priority
func Info(parts ...interface{}) {
	baseLog(InfoLevel, 4, parts...)
}

// Verbose performs Log with InfoLevel priority
func Verbose(parts ...interface{}) {
	baseLog(VerboseLevel, 4, parts...)
}

// Info performs Log with InfoLevel priority
func Warn(parts ...interface{}) {
	baseLog(WarningLevel, 4, parts...)
}

// func Dummy(parts ...interface{}) {
// }

// Debug performs Log with DebugLevel priority
func Debug(parts ...interface{}) {
	baseLog(DebugLevel, 4, parts...)
}

// Error performs Log with ErrorLevel priority, getting stack from N
func ErrorAtStack(stackPos int, parts ...interface{}) error {
	return baseLog(ErrorLevel, stackPos, parts...)
}

func expandTildeInFilepath(path string) string { // can't use one in zfile, cyclic dependency
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
		err = fmt.Errorf("%w %s", err, p)
	} else {
		err = errors.New(p)
	}
	return err
}

func baseLog(priority Priority, pos int, parts ...any) error {
	if priority < PrintPriority {
		return nil
	}
	if priority < WarningLevel && isInTests {
		return nil
	}
	for i := 0; i < len(parts); i++ {
		p := parts[i]
		n, got := p.(StackAdjust)
		if got {
			parts = append(parts[:i], parts[i+1:]...)
			pos += int(n)
		}
		t, got := p.(time.Time)
		if got {
			parts[i] = t.Local().Format("06-Jan-02 15:04:05.999-07")
		}
		rl, got := p.(LimitID)
		if got {
			parts = append(parts[:i], parts[i+1:]...) // can't use zslice as it would create cyclical import
			i--
			t, _ := rateLimiters.Get(rl)
			if time.Since(t) < time.Second {
				return nil
			}
			rateLimiters.Set(rl, time.Now())
		}
		enable, got := p.(Enabler)
		if got {
			if !bool(enable) {
				return nil
			}
			parts = append(parts[:i], parts[i+1:]...) // can't use zslice as it would create cyclical import
			i--
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
	if true { //runtime.GOOS != "js" {
		timeLock.Lock()
		linesPrintedSinceTimeStamp++
		num := runtime.NumGoroutine()
		if PrintGoRoutines && lastGoRoutineCount != num && time.Since(lastGoRoutineOutputTime) > time.Second*10 {
			finfo = fmt.Sprintln("goroutines:", num)
			lastGoRoutineCount = num
			lastGoRoutineOutputTime = time.Now()
		}
		if PrintDate {
			str := time.Now().Local().Format("15:04:05.00-02-01 ")
			if UseColor {
				str = zstr.EscCyan + str + zstr.EscNoColor
			}
			finfo += str
		}
		linesPrintedSinceTimeStamp = 0
		timeLock.Unlock()
	}
	if priority == DebugLevel {
		finfo += CallingFunctionString(pos) + ": "
	} else if priority == ErrorLevel {
		finfo += FileLineAndCallingFunctionString(pos) + ": "
	}
	if priority == FatalLevel {
		finfo += "\nFatal:" + CallingStackString() + "\n"
	}
	err := NewError(parts...)
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

func CallingFunctionInfo(pos int) (function, file string, line int) {
	pc, file, line, ok := runtime.Caller(pos)
	if ok {
		function = runtime.FuncForPC(pc).Name()
	}
	return
}

func CallingStackString() string {
	return CallingStackStringAt(1)
}

func CallingStackStringAt(index int) string {
	var parts []string
	for i := 3 + index; ; i++ {
		s := FileLineAndCallingFunctionString(i)
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

func CallingFunctionString(pos int) string {
	function, _, _ := CallingFunctionInfo(pos)
	return zstr.TailUntil(function, "/")
}

func FileLineAndCallingFunctionString(pos int) string {
	function, file, line := CallingFunctionInfo(pos)
	if function == "" {
		return ""
	}
	_, function = path.Split(function)

	wd, _ := os.Getwd()
	file = makePathRelativeTo(file, wd)

	return fmt.Sprintf("%s:%d %s()", file, line, function)
}

func Assert(success bool, parts ...interface{}) {
	if !success {
		parts = append([]interface{}{"assert:"}, parts...)
		fmt.Println(parts...)
		Fatal(parts...)
	}
}

func AssertMakeError(success bool, parts ...interface{}) error {
	if !success {
		parts = append([]interface{}{"assert failed:", StackAdjust(1)}, parts...)
		return Error(parts...)
	}
	return nil
}

func ErrorIf(check bool, parts ...interface{}) bool {
	if check {
		parts = append([]interface{}{"error if occured:", StackAdjust(1)}, parts...)
		Error(parts...)
	}
	return check
}

func OnError(err error, parts ...interface{}) bool {
	if err != nil {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Error(parts...)
		return true
	}
	return false
}

func AssertNotError(err error, parts ...interface{}) {
	if err != nil {
		parts = append([]interface{}{StackAdjust(1)}, parts...)
		Fatal(parts...)
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
	return fmt.Errorf("%w %s", err, p)

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

func HandlePanic(exit bool) error {
	if PanicHandler == nil {
		return nil
	}
	if runtime.GOOS == "js" {
		return nil
	}
	r := recover()
	if r != nil {
		fmt.Println("**HandlePanic")
		Error("\n🟥HandlePanic:", r, "\n", CallingStackString())
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

func OnErrorTestError(t *testing.T, err error, items ...interface{}) bool {
	if err != nil {
		items = append([]interface{}{err}, items...)
		t.Error(items...)
		return true
	}
	return false
}

func Full(v interface{}) string {
	return fmt.Sprintf("%+v", v)
}

func Pointer(v interface{}) string {
	return fmt.Sprintf("%p", v)
}

func Hex(v interface{}) string {
	return fmt.Sprintf("%X", v)
}

func Limit(parts ...any) LimitID {
	return LimitID(fmt.Sprint(parts...))
}

func Func() {
	Info(CallingFunctionString(3))
}

func RegisterEnabler(name string, b *Enabler) {
	EnablerList.Set(name, b)
}
