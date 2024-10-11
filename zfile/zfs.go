//go:build !js

package zfile

import (
	"bytes"
	"errors"
	"io"
	"io/fs"

	"github.com/torlangballe/zutil/zbytes"
)

type multiRow struct {
	FS     fs.FS
	FSName string
}

type MultiFS []multiRow

func (m MultiFS) OpenReturningFSName(filename string) (fs.File, string, error) {
	for _, f := range m {
		file, err := f.FS.Open(filename)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return file, f.FSName, err
		}
	}
	return nil, "", fs.ErrNotExist
}

func (m MultiFS) Open(filename string) (fs.File, error) {
	f, _, err := m.OpenReturningFSName(filename)
	return f, err
}

func (m MultiFS) IsOpenable(filename string) (bool, string) {
	f, fsname, _ := m.OpenReturningFSName(filename)
	if f != nil {
		f.Close()
		return true, fsname
	}
	return false, ""
}

func CanOpenInFS(f fs.FS, filename string) bool {
	file, err := f.Open(filename)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// Stat does not work for embeded file systems, so is not a good way to detect if file present.
func (m MultiFS) Stat(filename string) (fs.FileInfo, string, error) {
	for _, f := range m {
		stat, got := f.FS.(fs.StatFS)
		if !got {
			continue
		}
		info, err := stat.Stat(filename)
		// zlog.Info("fs.Stat", name, err)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return info, f.FSName, err
		}
	}
	return nil, "", fs.ErrNotExist
}

func (m *MultiFS) Add(f fs.FS, fsname string) {
	var row multiRow
	row.FS = f
	row.FSName = fsname
	*m = append(*m, row)
}

func (m *MultiFS) InsertFirst(f fs.FS, fsname string) {
	var row multiRow
	row.FS = f
	row.FSName = fsname
	*m = append([]multiRow{row}, *m...)
}

func CountBytesOfFileInFS(f fs.FS, name string) (int64, error) {
	file, err := f.Open(name)
	if err != nil {
		return 0, err
	}
	return zbytes.CountReadUntilError(file)
}

func ReadBytesFromFileInFS(f fs.FS, name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	buffer := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buffer, file)
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func ReadStringFromFileInFS(f fs.FS, name string) (string, error) {
	data, err := ReadBytesFromFileInFS(f, name)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ReaderAtFromFileInFS reads entire file to memory to be able to create a io.ReaderAt
func ReaderAtFromFileInFS(f fs.FS, name string) (reader io.ReaderAt, length int64, err error) {
	data, err := ReadBytesFromFileInFS(f, name)
	if err != nil {
		return nil, 0, err
	}
	reader = bytes.NewReader(data)
	return reader, int64(len(data)), nil
}

func ReaderFromFileInFS(f fs.FS, name string) (reader io.Reader, err error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	return file, nil
	// data, err := ReadBytesFromFileInFS(f, name)
	// if err != nil {
	// 	return nil, 0, err
	// }
	// reader = bytes.NewReader(data)
	// return reader, int64(len(data)), nil
}
