//go:build !js

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
)

type KeyValueRPCCalls struct{}

var (
	storePath string
	rpcStore  = zkeyvalue.NewFileStore("")
)

func Init(storePath string) {
	rpcStore.Load(storePath)
}

func NewOption[V any](key string, val V) *zkeyvalue.Option[V] {
	return zkeyvalue.NewOption[V](rpcStore, key, val)
}

func (KeyValueRPCCalls) ReadStore(ci *zrpc.ClientInfo, in *zrpc.Unused, store *zdict.Dict) error {
	*store = rpcStore.GetAllForPrefix(ci.Token + "/")
	zlog.Info("zkeyvalrpc ReadStore:", store)
	return nil
}

func (KeyValueRPCCalls) SetItem(ci *zrpc.ClientInfo, kv zdict.Item, result *zrpc.Unused) error {
	rpcStore.SetItem(ci.Token+"/"+kv.Name, kv.Value, true)
	return nil
}

// func (s Store) GetItem(key string, pointer interface{}) bool {
// 	gval, got := s.GetItemAsAny(key)
// 	if got {
// 		reflect.ValueOf(pointer).Elem().Set(reflect.ValueOf(gval))
// 		return true
// 	}
// 	return false
// }

// func (s Store) GetItemAsAny(key string) (any, bool) {
// 	s.postfixKey(&key)
// 	lock.Lock()
// 	defer lock.Unlock()
// 	gval, got := dict[key]
// 	return gval, got
// }

// func (s *Store) SetItem(key string, v any, sync bool) error {
// 	s.postfixKey(&key)
// 	lock.Lock()
// 	dict[key] = v
// 	lock.Unlock()
// 	if sync {
// 		s.save()
// 	}
// 	return nil
// }

// func (s *Store) save() error {
// 	lock.Lock()
// 	err := zjson.MarshalToFile(dict, s.path)
// 	zlog.OnError(err, "save", s.path)
// 	lock.Unlock()
// 	return err
// }

// func (s *Store) RemoveForKey(key string, sync bool) {
// 	s.postfixKey(&key)
// 	lock.Lock()
// 	delete(dict, key)
// 	lock.Unlock()
// 	if sync {
// 		s.save()
// 	}
// }
