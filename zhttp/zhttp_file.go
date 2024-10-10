//go:build !js

package zhttp

import (
	"hash/crc32"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
)

// ReceiveToFile receives a file as a POST body, to path or temporary file.
// The file's crc and length are calculated and compared with values sent with SendForFile()
func ReceiveToFile(req *http.Request, toPath string, inTemp bool) (path string, err error) {
	path = toPath
	if inTemp {
		path = zfile.CreateTempFilePath(toPath)
	}
	file, err := os.Create(path)
	if err != nil {
		return path, err
	}
	crcWriter := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	multi := io.MultiWriter(file, crcWriter)
	defer file.Close()
	n, err := io.Copy(multi, req.Body)
	if err != nil {
		return path, zlog.Error("copy from request to file/crc", err)
	}
	conLen, cerr := GetContentLengthFromHeader(req.Header)
	calcCRC := int64(crcWriter.Sum32())
	headerCRC, perr := strconv.ParseInt(req.Header.Get("X-ZCRC"), 10, 64)
	if calcCRC == 0 || perr != nil || cerr != nil || calcCRC != headerCRC || conLen != n {
		return path, zlog.Error("incomplete upload", headerCRC, "!=", calcCRC, perr)
	}
	return path, nil
}
