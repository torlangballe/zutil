package zfilelog

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sync"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/ztimer"
)

var truncatingLock sync.Mutex

type Truncater struct {
	Truncating bool
	Buffer     string
	Repeater   *ztimer.Repeater
}

var truncaters = map[string]*Truncater{}

func writeText(addText, fpath string) error {
	file, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	file.WriteString(addText)
	file.Close()
	return nil
}

func AddToLogFile(fpath string, addText string) error {
	folder, _ := filepath.Split(fpath)
	if folder != "" {
		os.MkdirAll(folder, 0775|os.ModeDir)
	}
	truncatingLock.Lock()
	t := truncaters[fpath]
	if t == nil {
		t = &Truncater{}
		t.Repeater = ztimer.RepeatIn(3600+200*rand.Float64(), func() bool {
			truncatingLock.Lock()
			size := zfile.Size(fpath)
			truncaters[fpath].Truncating = true
			err := zfile.TruncateFile(fpath, 20*1024*1024, 0.9, false) //
			if err != nil {
				fmt.Println("{nolog}zlog.AddToLogFile TruncateFile err:", err)
			}
			t := truncaters[fpath]
			writeText(t.Buffer, fpath)
			t.Buffer = ""
			t.Truncating = false
			nsize := zfile.Size(fpath)
			if nsize < size {
				fmt.Println("{nolog}Truncated Log:", fpath, size, "->", nsize)
			}
			truncatingLock.Unlock()
			return true
		})
		truncaters[fpath] = t
	} else {
		if t.Truncating {
			t.Buffer += addText
		} else {
			writeText(addText, fpath)
		}
	}
	truncatingLock.Unlock()
	return nil
}

func ReadFromLogFileEnd(fpath string, endPos int64, lines int) (out string, fileSize int64, err error) {
	var pos int64
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
