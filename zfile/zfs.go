package zfile

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
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

func (m *MultiFS) Add(f fs.FS) {
	*m = append(*m, f)
}

func ReadBytesFromFileInFS(f fs.FS, name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	buff := bytes.NewBuffer([]byte{})
	_, err = io.Copy(buff, file)
	if err != nil {
		return nil, err
	}
	return buff.Bytes(), nil
}
