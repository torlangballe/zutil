package zdebug

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zwords"
)

const ProfilingURLPrefix = "debug/pprof/"

var (
	IsInTests                = (strings.HasSuffix(os.Args[0], ".test"))
	GetOpenFileCountFunc     func() int
	AverageCPUFunc           func() float64
	GetOngoingProcsCountFunc func() int
	TimersGoingCountFunc     func() int
	AllProfileTypes          = []string{"heap", "profile", "block", "mutex"}
)

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
	fmt.Printf("MemAlloc@%s: %s TotalAlloc:%s Sys:%s RSS:%s CPU:%d NumGC:%d Gos:%d Files:%d Repeaters:%d Ongoing:%d\n", time.Now().Local().Format("15:04"), memStr(m.Alloc), memStr(m.TotalAlloc), memStr(m.Sys), memStr(rss), cpu, m.NumGC, goroutines, files, TimersGoingCountFunc, procsCount)
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

func FileLineAndCallingFunctionString(pos int, shortFile bool) string {
	function, file, line := CallingFunctionInfo(pos)
	if function == "" {
		return ""
	}
	_, function = path.Split(function)
	if shortFile {
		_, file = path.Split(file)
	} else {
		wd, _ := os.Getwd()
		file = makePathRelativeTo(file, wd)
	}
	return fmt.Sprintf("%s:%d %s()", file, line, function)
}
