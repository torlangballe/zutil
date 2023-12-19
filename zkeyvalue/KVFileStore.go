//go:build !js

package zkeyvalue

import (
	"strings"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
)

type FileStore struct {
	Store
	storeFile string
}

var (
	DefaultStore *FileStore
)

func NewFileStore(path string) *FileStore {
	s := &FileStore{}
	s.Raw = NewDictRawStore()
	if path != "" {
		s.Load(path)
	}
	return s
}

func (s *FileStore) Load(path string) error {
	drs := s.Raw.(*DictRawStore)
	s.storeFile = zfile.ChangedExtension(path, ".json")
	err := zjson.UnmarshalFromFile(&drs.dict, s.storeFile, true)
	if err != nil {
		return zlog.Error(err, "unmarshal")
	}
	return nil
}

func (s *FileStore) save() error {
	fsr := s.Raw.(*DictRawStore)
	fsr.lock.Lock()
	err := zjson.MarshalToFile(fsr.dict, s.storeFile)
	zlog.OnError(err, "save", s.storeFile)
	fsr.lock.Unlock()
	return err
}

func (s *FileStore) GetAllForPrefix(prefix string) zdict.Dict {
	fsr := s.Raw.(*DictRawStore)
	d := zdict.Dict{}
	fsr.lock.Lock()
	for k, v := range fsr.dict {
		if strings.HasPrefix(k, prefix) {
			d[k] = v
		}
	}
	fsr.lock.Unlock()
	return d
}

func (s *FileStore) SetItem(key string, v any, sync bool) error {
	fsr := s.Raw.(*DictRawStore)
	err := fsr.RawSetItem(key, v, sync)
	if err == nil {
		s.save()
	}
	return err
}

func (s *FileStore) RemoveForKey(key string, sync bool) {
	fsr := s.Raw.(*DictRawStore)
	fsr.RawRemoveForKey(key, sync)
	s.save()
}
