//go:build !js

package zkeyvalue

import (
	"path"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
)

type dictFile struct {
	DictRawStore
	storeFile string
}

func (d *dictFile) load() error {
	err := zjson.UnmarshalFromFile(&d.dict, d.storeFile, true)
	if err != nil {
		return zlog.Error(err, "unmarshal")
	}
	return nil
}

func NewFileStore(fpath string) *Store {
	s := &Store{}
	df := &dictFile{}
	df.storeFile = zfile.ChangedExtension(fpath, ".json")
	df.load()
	s.Saver = df
	s.Raw = df
	dir, _ := path.Split(fpath)
	zfile.MakeDirAllIfNotExists(dir)
	// zlog.Info("NewFileStore:", path, s.Raw, zlog.CallingStackString())
	return s
}

func (d *dictFile) Save() error {
	d.lock.Lock()
	err := zjson.MarshalToFile(d.dict, d.storeFile)
	zlog.OnError(err, "Save", d.storeFile, zlog.CallingStackString())
	d.lock.Unlock()
	return err
}

// func (s *FileStore) SetItem(key string, v any) error {
// 	err := s.DictRawStore().RawSetItem(key, v)
// 	if err == nil {
// 		s.Save()
// 	}
// 	return err
// }

// func (s *FileStore) RemoveForKey(key string, sync bool) {
// 	drs := s.DictRawStore()
// 	drs.RawRemoveForKey(key, sync)
// 	s.Save()
// }

// func (s *FileStore) DictRawStore() *DictRawStore {
// 	return s.Raw.(*DictRawStore)
// }
