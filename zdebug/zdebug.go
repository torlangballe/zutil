package zdebug

import (
	"fmt"
	"runtime"

	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zwords"
)

func memStr(m uint64) string {
	return zwords.GetMemoryString(int64(m), "", 1)
}

func PrintMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	rss := m.HeapSys - m.HeapReleased
	goroutines := runtime.NumGoroutine()
	files := zprocess.GetOpenFileCount()

	fmt.Printf("MemAlloc:%s TotalAlloc:%s Sys:%s RSS:%s NumGC:%d Gos:%d Files:%d\n", memStr(m.Alloc), memStr(m.TotalAlloc), memStr(m.Sys), memStr(rss), m.NumGC, goroutines, files)
}

func PrintAllGoroutines() {
	buf := make([]byte, 1<<16)
	runtime.Stack(buf, true)
	fmt.Printf("%s", buf)
}

func Consume(p ...any) {

}
