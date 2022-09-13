//go:build !js

package zkeyvalue

import (
	"reflect"
	"sync"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
)

var lock sync.Mutex
var dict = zdict.Dict{}

func StoreFileNew(path string) *Store {
	k := &Store{Local: true}
	k.filepath = zfile.ChangedExtension(path, ".json")
	err := zjson.UnmarshalFromFile(&dict, k.filepath, true)
	if err != nil {
		zlog.Error(err, "unmarshal")
		return nil
	}
	return k
}

func (k Store) getItem(key string, pointer interface{}) bool {
	if key[0] != '/' && k.KeyPrefix != "" {
		key = k.KeyPrefix + "/" + key
	}
	lock.Lock()
	defer lock.Unlock()
	gval, got := dict[key]
	if got {
		reflect.ValueOf(pointer).Elem().Set(reflect.ValueOf(gval))
		return true
	}
	return false
}

func (k *Store) setItem(key string, v interface{}, sync bool) error {
	go func() {
		lock.Lock()
		dict[key] = v
		err := zjson.MarshalToFile(dict, k.filepath)
		if err != nil {
			zlog.Error(err, "marshal")
		}
		lock.Unlock()
	}()
	return nil
}
