package zdebug

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zwords"
)

const (
	ProfilingURLPrefix     = "debug/pprof/"
	restartContextErrorKey = "zdebug.RestartContextError"
)

var (
	IsInTests                            = (strings.HasSuffix(os.Args[0], ".test"))
	GetOpenFileCountFunc                 func() int
	AverageCPUFunc                       func() float64
	GetOngoingProcsCountFunc             func() int
	TimersGoingCountFunc                 func() int
	MakeContextErrorForSignalRestartFunc func(pos int, invokeFunc string, parts ...any) error
	KeyValueSaveContextErrorFunc         func(key string, object any)
	KeyValueGetAndDeleteContextErrorFunc func(key string) (err error)

	AllProfileTypes = []string{"heap", "cpu", "block", "mutex"}

	HandleRestartFunc func(err error)
)

func init() {
	HandleRestartFunc = func(err error) {
		fmt.Println("fatal handler:", err)
	}
}

func memStr(m uint64) string {
	return zwords.GetMemoryString(int64(m), "", 1)
}

func PrintMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	rss := m.HeapSys - m.HeapReleased
	goroutines := runtime.NumGoroutine()
	files := -1
	if GetOpenFileCountFunc != nil {
		files = GetOpenFileCountFunc()
	}
	procsCount := GetOngoingProcsCountFunc()
	cpu := int(AverageCPUFunc() * 100)
	fmt.Printf("MemAlloc@%s: %s TotalAlloc:%s Sys:%s RSS:%s CPU:%d NumGC:%d Gos:%d Files:%d Repeaters:%d Ongoing:%d\n", time.Now().Local().Format("15:04"), memStr(m.Alloc), memStr(m.TotalAlloc), memStr(m.Sys), memStr(rss), cpu, m.NumGC, goroutines, files, TimersGoingCountFunc(), procsCount)
}

func PrintAllGoroutines() {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Printf("%s", buf)
}

func Consume(p ...any) {

}

func CallingFunctionInfo(pos int) (function, file string, line int) {
	pc, file, line, ok := runtime.Caller(pos)
	if ok {
		function = runtime.FuncForPC(pc).Name()
	}
	return
}

func CallingFunctionShortInfo(pos int) (function, shortFunction, file, shortFile string, line int) {
	var ok bool
	var pc uintptr

	pc, file, line, ok = runtime.Caller(pos)
	if !ok {
		return "", "", "", "", 0
	}
	function = runtime.FuncForPC(pc).Name()
	_, shortFunction = path.Split(function)
	_, shortFile = path.Split(file)
	return function, shortFunction, file, shortFile, line
}

func CallingStackString() string {
	return CallingStackStringAt(1)
}

func CallingStackStringAt(index int) string {
	var parts []string
	for i := 3 + index; ; i++ {
		s := FileLineAndCallingFunctionString(i, false)
		if s == "" {
			break
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n")
}

func CallingFunctionString(pos int) string {
	function, _, _ := CallingFunctionInfo(pos)
	return zstr.TailUntil(function, "/")
}

func FileLineAndCallingFunctionString(pos int, useShortFile bool) string {
	function, shortFunction, file, shortFile, line := CallingFunctionShortInfo(pos)
	if function == "" {
		return ""
	}
	if useShortFile {
		file = shortFile
	}
	return fmt.Sprintf("%s:%d %s()", file, line, shortFunction)
}

func RecoverFromPanic(exit bool, invokeFunc string) {
	upStackLevels := 4
	if runtime.GOOS == "js" {
		upStackLevels = 5
	}
	r := recover()
	if r == nil {
		return
	}
	err, _ := r.(error)
	if err == nil {
		err = fmt.Errorf("%v", r)
	}

	err = MakeContextErrorForSignalRestartFunc(upStackLevels, invokeFunc, "Panic Restart", err)
	fmt.Println("RecoverFromPanic:", upStackLevels, err, r)
	debug.PrintStack()
	StoreAndExitError(err, exit)
}

func StoreAndExitError(err error, exit bool) {
	KeyValueSaveContextErrorFunc(restartContextErrorKey, err)
	if exit {
		os.Exit(-1)
	}
	HandleRestartFunc(err)
}

func LoadStoredRestartContextError() {
	err := KeyValueGetAndDeleteContextErrorFunc(restartContextErrorKey)
	// fmt.Printf("LoadStoredRestartContextError %+v\n:", err)
	// if err != nil {
	HandleRestartFunc(err)
	// }
}
