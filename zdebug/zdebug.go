package zdebug

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwords"
)

const ProfilingURLPrefix = "debug/pprof/"

var (
	IsInTests            = (strings.HasSuffix(os.Args[0], ".test"))
	GetOpenFileCountFunc func() int
	AllProfileTypes      = []string{"heap", "profile", "block", "mutex"}
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
	fmt.Printf("MemAlloc@%s: %s TotalAlloc:%s Sys:%s RSS:%s NumGC:%d Gos:%d Files:%d Repeaters:%d\n", time.Now().Local().Format("15:04"), memStr(m.Alloc), memStr(m.TotalAlloc), memStr(m.Sys), memStr(rss), m.NumGC, goroutines, files, ztimer.GoingCount)
}

func PrintAllGoroutines() {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Printf("%s", buf)
}

func Consume(p ...any) {

}
