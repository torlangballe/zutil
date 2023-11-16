package zdebug

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/torlangballe/zutil/zwords"
)

var (
	IsInTests            = (strings.HasSuffix(os.Args[0], ".test"))
	ProfilingPort        int
	GetOpenFileCountFunc func() int
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
	fmt.Printf("MemAlloc:%s TotalAlloc:%s Sys:%s RSS:%s NumGC:%d Gos:%d Files:%d\n", memStr(m.Alloc), memStr(m.TotalAlloc), memStr(m.Sys), memStr(rss), m.NumGC, goroutines, files)
}

func PrintAllGoroutines() {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Printf("%s", buf)
}

func Consume(p ...any) {

}

func GetProfileCommandLineGetters(addressIP4 string) []string {
	var out []string
	for _, n := range []string{"heap", "profile", "block", "mutex"} {
		str := fmt.Sprintf("curl http://%s:%d/debug/pprof/%s > ~/%s && go tool pprof -web ~/%s", addressIP4, ProfilingPort, n, n, n)
		out = append(out, str)
	}
	return out
}
