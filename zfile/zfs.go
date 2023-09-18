package zfile

import (
	"bytes"
	"errors"
	"io"
	"io/fs"

	"github.com/torlangballe/zutil/zlog"
)

type MultiFS []fs.FS

func (m MultiFS) Open(name string) (fs.File, error) {
	for _, f := range m {
		file, err := f.Open(name)
		// zlog.Info("fs.Open", name, err)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return file, err
		}
	}
	return nil, fs.ErrNotExist
}

func CanOpenInFS(f fs.FS, name string) bool {
	file, err := f.Open(name)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

func (m MultiFS) Stat(name string) (fs.FileInfo, error) {
	for _, f := range m {
		is, i := f.(fs.StatFS)
		zlog.Assert(is != nil, i)
		info, err := is.Stat(name)
		zlog.Info("fs.Stat", name, err)
		if err == nil || !errors.Is(err, fs.ErrNotExist) {
			return info, err
		}
	}
	return nil, fs.ErrNotExist
}

func (m *MultiFS) Add(f fs.FS) {
	*m = append(*m, f)
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

func ReaderAtFromFileInFS(f fs.FS, name string) (reader io.ReaderAt, length int64, err error) {
	data, err := ReadBytesFromFileInFS(f, name)
	if err != nil {
		return nil, 0, err
	}
	reader = bytes.NewReader(data)
	return reader, int64(len(data)), nil
}

func ReaderFromFileInFS(f fs.FS, name string) (reader io.Reader, length int64, err error) {
	data, err := ReadBytesFromFileInFS(f, name)
	if err != nil {
		return nil, 0, err
	}
	reader = bytes.NewReader(data)
	return reader, int64(len(data)), nil
}
