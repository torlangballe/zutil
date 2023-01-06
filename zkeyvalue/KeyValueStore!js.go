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

// This is a hack that only has one global store!

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
	zlog.Info("KV: StoreFileNew", path, dict)
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

func (s *Store) setItem(key string, v interface{}, sync bool) error {
	s.prefixKey(&key)
	lock.Lock()
	dict[key] = v
	lock.Unlock()
	if sync {
		s.save()
	}
	return nil
}

func (s *Store) save() error {
	lock.Lock()
	err := zjson.MarshalToFile(dict, s.filepath)
	zlog.OnError(err, "save", s.filepath)
	lock.Unlock()
	return err
}

func (s *Store) RemoveForKey(key string, sync bool) {
	s.prefixKey(&key)
	lock.Lock()
	delete(dict, key)
	lock.Unlock()
	if sync {
		s.save()
	}
}
