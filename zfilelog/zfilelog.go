//go:build !js

package zfilelog

import (
	"github.com/torlangballe/zutil/zfile"
)

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
