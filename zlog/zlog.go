package zlog

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
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
	PrintPriority              = DebugLevel
	outputHooks                = map[string]func(s string){}
	UseColor                   = false
	PrintDate                  = true
	EnablerList                sync.Map // map[string]*Enabler
	rateLimiters               sync.Map
	isInTests                  = (strings.HasSuffix(os.Args[0], ".test"))
	hookingLock                sync.Mutex
	hooking                    = false
	timeLock                   sync.Mutex
	linesPrintedSinceTimeStamp int
)

func init() {
	zdict.AssertFunc = Assert
	zdebug.MakeContextErrorForSignalRestartFunc = func(pos int, invokeFunc string, parts ...any) error {
		err := NewLogError(FatalLevel, time.Now(), pos, parts...)
		if invokeFunc == "" {
			return err
		}
		ce, is := err.(zerrors.ContextError)
		if !is {
			return err
		}
		ce.SetKeyValue("Invoked", invokeFunc)
		return ce
	}
}

func Error(parts ...any) error {
	return baseLog(ErrorLevel, 4, parts...)
}

// Fatal performs Log with Fatal priority
func Fatal(parts ...any) error {
	return baseLog(FatalLevel, 4, parts...)
}

func FatalNotImplemented() {
	Fatal(nil, "Not Implemented")
}

// Info performs Log with InfoLevel priority
func Info(parts ...any) {
	baseLog(InfoLevel, 4, parts...)
}

// Info performs Log with InfoLevel priority
func Infof(format string, parts ...any) {
	str := fmt.Sprintf(format, parts...)
	baseLog(InfoLevel, 4, str)
}

// Verbose performs Log with InfoLevel priority
func Verbose(parts ...any) {
	baseLog(VerboseLevel, 4, parts...)
}

// Info performs Log with InfoLevel priority
func Warn(parts ...any) {
	baseLog(WarningLevel, 4, parts...)
}

// func Dummy(parts ...any) {
// }

// Debug performs Log with DebugLevel priority
func Debug(parts ...any) {
	baseLog(DebugLevel, 4, parts...)
}

// Error performs Log with ErrorLevel priority, getting stack from N
func ErrorAtStack(stackPos int, parts ...any) error {
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

func NewError(parts ...any) error {
	return NewLogError(InfoLevel, time.Now(), 1, parts...)
}

func makeContextErrorFunc(m map[string]any, parts ...any) error {
	dict := zdict.FromShallowMap(m)
	ce := zerrors.MakeContextError(dict, parts...)
	return ce
}

func baseLog(priority Priority, pos int, parts ...any) error {
	if priority < PrintPriority {
		return nil
	}
	if priority < WarningLevel && isInTests {
		return nil
	}
	now := time.Now()
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
			parts = append(parts[:i], parts[i+1:]...) // can't use zslice.RemoveAt() as it would create cyclical import
			i--
			tl, _ := rateLimiters.Load(rl)
			if tl != nil {
				t := tl.(time.Time)
				if time.Since(t) < time.Second {
					return nil
				}
			}
			rateLimiters.Store(rl, now)
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
		if PrintDate {
			str := now.Local().Format("15:04:05.00-02-01 ")
			if UseColor {
				str = zstr.EscCyan + str + zstr.EscNoColor
			}
			finfo += str
		}
		linesPrintedSinceTimeStamp = 0
		timeLock.Unlock()
	}
	if priority == DebugLevel {
		finfo += zdebug.CallingFunctionString(pos) + ": "
	} else if priority == ErrorLevel {
		finfo += zdebug.FileLineAndCallingFunctionString(pos, false) + ": "
	}
	if priority == FatalLevel {
		finfo += "\nFatal:" + zdebug.CallingStackString() + "\n"
	}
	var cparts, nparts []any
	for _, p := range parts {
		n := p
		s, got := p.(string)
		if got {
			var did bool
			s, did = zstr.EscapeColorSymbols(s)
			if did {
				endCol = zstr.EscNoColor
				p = s
			}
			e, did := zstr.RemoveColorSymbols(s)
			if did {
				n = e
			}
		}
		cparts = append(cparts, p)
		nparts = append(nparts, n)
	}
	colStr := zstr.Spaced(cparts...)
	var err error
	if priority >= ErrorLevel {
		err = NewLogError(priority, now, pos, nparts...)
	}
	fmt.Println(finfo + col + colStr + endCol)
	whiteStr := finfo + zstr.Spaced(nparts...) + "\n"
	hookingLock.Lock()
	if !hooking {
		hooking = true
		for _, f := range outputHooks {
			f(whiteStr)
		}
		hooking = false
	}
	hookingLock.Unlock()
	writeToSyslog(whiteStr)
	if priority == FatalLevel {
		Info("Log fatal exit!")
		zdebug.StoreAndExitError(err, true)
	}
	return err
}

func NewLogError(priority Priority, time time.Time, pos int, parts ...any) error {
	dict := map[string]any{}
	if priority >= ErrorLevel {
		function, _, file, _, line := zdebug.CallingFunctionShortInfo(pos + 1)
		dict["Func"] = function
		dict["Code File"] = zstr.CodeLink(fmt.Sprintf("%s:%d", file, line))
		dict["Time"] = time
		dict["Stack"] = zstr.TruncatedFromEnd(zdebug.CallingStackStringAt(pos), 10*1024, "\n---call stack truncated at 10K---")
	}
	err := makeContextErrorFunc(dict, parts...)

	return err
}

func Assert(success bool, parts ...any) {
	if !success {
		parts = append([]any{"Assert"}, parts...) // StackAdjust(1),
		Fatal(parts...)
	}
}

func AssertMakeError(success bool, parts ...any) error {
	if !success {
		parts = append([]any{"Assert failed:", StackAdjust(1)}, parts...)
		return Error(parts...)
	}
	return nil
}

func ErrorIf(check bool, parts ...any) bool {
	if check {
		parts = append([]any{"error if occured:", StackAdjust(1)}, parts...)
		Error(parts...)
	}
	return check
}

func OnError(err error, parts ...any) bool {
	if err != nil {
		parts = append([]any{StackAdjust(1)}, parts...)
		parts = append(parts, err)
		Error(parts...)
		return true
	}
	return false
}

func AssertNotError(err error, parts ...any) {
	if err != nil {
		parts = append([]any{StackAdjust(1), err}, parts...)
		Fatal(parts...)
	}
}

func AddHook(id string, call func(s string)) {
	outputHooks[id] = call
}

// type WrappedError struct {
// 	Text  string
// 	Error error
// }

// func (w *WrappedError) Unwrap() error {
// 	return w.Error
// }

// func Wrap(err error, parts ...any) error {
// 	p := strings.TrimSpace(fmt.Sprintln(parts...))
// 	return fmt.Errorf("%w %s", err, p)

// }

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

func OnErrorTestError(t *testing.T, err error, items ...any) bool {
	if err != nil {
		items = append([]any{err}, items...)
		t.Error(items...)
		return true
	}
	return false
}

func Full(v any) string {
	return fmt.Sprintf("%+v", v)
}

func Pointer(v any) string {
	return fmt.Sprintf("%p", v)
}

func Hex(v any) string {
	return fmt.Sprintf("%X", v)
}

func Limit(parts ...any) LimitID {
	return LimitID(fmt.Sprint(parts...))
}

func Func() {
	Info(zdebug.CallingFunctionString(3))
}

func RegisterEnabler(name string, b *Enabler) {
	EnablerList.Store(name, b)
}
