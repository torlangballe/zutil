package zfilelog

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

var mapLock sync.Mutex
var writeLocks = map[string]*sync.Mutex{}

func getLock(fpath string) *sync.Mutex {
	mapLock.Lock()
	lock := writeLocks[fpath]
	mapLock.Unlock()
	if lock == nil {
		lock = &sync.Mutex{}
		writeLocks[fpath] = lock
		ztimer.RepeatIn(3600+200*rand.Float64(), func() bool {
			lock.Lock()
			size := zfile.Size(fpath)
			err := zfile.TruncateFile(fpath, 20*1024*1024, 0.9, false) //
			if err != nil {
				fmt.Println("{nolog}zlog.AddToLogFile TruncateFile err:", err)
			}
			nsize := zfile.Size(fpath)
			if nsize < size {
				fmt.Println("{nolog}Truncated Log:", fpath, size, "->", nsize)
			}
			lock.Unlock()
			return true
		})
	}
	return lock
}

func AddToLogFile(fpath string, addText string) error {
	folder, _ := filepath.Split(fpath)
	if folder != "" && !zfile.Exists(folder) {
		os.MkdirAll(folder, 0775|os.ModeDir)
	}
	// zlog.Info("AddToLog", fpath, ":", strings.TrimSpace(addText))
	lock := getLock(fpath)
	lock.Lock()
	file, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return zlog.Error(err, "open", fpath)
	}
	file.WriteString(addText)
	file.Close()
	lock.Unlock()
	return nil
}

func ReadFromLogFileEnd(fpath string, endPos int64, lines int) (out string, fileSize int64, err error) {
	var pos int64
	lock := getLock(fpath)
	lock.Lock()
	defer lock.Unlock()
	fileSize = zfile.Size(fpath)
	count := 0
	for {
		str, size, newPos, rerr := zfile.ReadLastLine(fpath, pos)
		if rerr != nil {
			return "", size, rerr
		}
		if pos == 0 {
			fileSize = size
		}
		out += str + "\n"
		if newPos == 0 || pos <= endPos {
			break
		}
		if count >= lines {
			break
		}
		pos = newPos
	}
	return
}

func ReadFromLogAtPosition(fpath string, pos int64, lines int) (out string, newPos int64, err error) {
	lock := getLock(fpath)
	lock.Lock()
	defer lock.Unlock()
	newPos = pos
	count := 0
	for {
		var str string
		str, _, newPos, err = zfile.ReadLastLine(fpath, newPos)
		if err != nil {
			return
		}
		out += str + "\n"
		count++
		if newPos == 0 || count >= lines {
			break
		}
	}
	return
}
