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
	s.storeFile = zfile.ChangedExtension(path, ".json")
	if path != "" {
		s.Load(path)
	}
	// zlog.Info("NewFileStore:", path, s.Raw, zlog.CallingStackString())
	return s
}

func (s *FileStore) Load(path string) error {
	drs := s.DictRawStore()
	err := zjson.UnmarshalFromFile(&drs.dict, s.storeFile, true)
	if err != nil {
		return zlog.Error(err, "unmarshal")
	}
	return nil
}

func (s *FileStore) Save() error {
	drs := s.DictRawStore()
	drs.lock.Lock()
	err := zjson.MarshalToFile(drs.dict, s.storeFile)
	zlog.OnError(err, "Save", s.storeFile, zlog.CallingStackString())
	drs.lock.Unlock()
	return err
}

func (s *FileStore) GetAllForPrefix(prefix string) zdict.Dict {
	drs := s.DictRawStore()
	d := zdict.Dict{}
	drs.lock.Lock()
	for k, v := range drs.dict {
		if strings.HasPrefix(k, prefix) {
			d[k] = v
		}
	}
	drs.lock.Unlock()
	return d
}

func (s *FileStore) SetItem(key string, v any, sync bool) error {
	err := s.DictRawStore().RawSetItem(key, v, sync)
	if err == nil {
		s.Save()
	}
	return err
}

func (s *FileStore) RemoveForKey(key string, sync bool) {
	drs := s.DictRawStore()
	drs.RawRemoveForKey(key, sync)
	s.Save()
}

func (s *FileStore) DictRawStore() *DictRawStore {
	return s.Raw.(*DictRawStore)
}
