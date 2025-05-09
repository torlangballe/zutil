//go:build !js

package zkeyvalue

import (
	"path"

	"github.com/torlangballe/zutil/zdebug"
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
		return zlog.Error("unmarshal", err)
	}
	return nil
}

func NewFileStore(fpath string) *Store {
	s := &Store{}
	df := &dictFile{}
	df.DictRawStore = *NewDictRawStore()
	df.storeFile = zfile.ChangedExtension(fpath, ".json")
	df.load()
	s.Saver = df
	s.Raw = df
	dir, _ := path.Split(fpath)
	zfile.MakeDirAllIfNotExists(dir)
	return s
}

func (d *dictFile) Save() error {
	d.lock.Lock()
	err := zjson.MarshalToFile(d.dict, d.storeFile)
	zlog.OnError(err, "Save", d.storeFile, zdebug.CallingStackString())
	d.lock.Unlock()
	return err
}
