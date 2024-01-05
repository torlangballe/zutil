//go:build zui

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
)

type RPCStore struct {
	zkeyvalue.Store
}

var rpcStore = newRPCStore()

func Init() {
	zrpc.MainClient.RegisterPollGetter(ResourceID, getAll)
	zrpc.RegisterResources(ResourceID)
}

func newRPCStore() *RPCStore {
	s := &RPCStore{}
	drs := zkeyvalue.NewDictRawStore()
	s.Store.Raw = drs
	return s
}

func getAll() {
	go func() {
		var dict zdict.Dict
		err := zrpc.MainClient.Call("KeyValueRPCCalls.GetAll", nil, &dict)
		if zlog.OnError(err) {
			return
		}
		old := rpcStore.Raw.(*zkeyvalue.DictRawStore).All()
		for k, v := range dict {
			oldVal, got := old[k]
			if !got || oldVal != v {
				externalChangeHandlers.ForAll(func(key string, f func(key string, value any, isLoad bool)) {
					f(k, v, true)
				})
				old[k] = v
			}
		}
		drs := rpcStore.Raw.(*zkeyvalue.DictRawStore)
		drs.Set(dict)

	}()
}

func NewOption[V comparable](key string, val V) *zkeyvalue.Option[V] {
	o := zkeyvalue.NewOption[V](rpcStore, key, val)
	AddExternalChangedHandler(key, func(inkey string, value any, isLoad bool) {
		if inkey == key {
			// zlog.Info("kvrpc got:", inkey, value, o.Get())
			o.SetAny(value, true)
			// zlog.Info("kvrpc got2:", inkey, value, o.Get())
		}
	})
	return o
}

func (s *RPCStore) GetItem(key string, pointer interface{}) bool {
	return false
}

func (s *RPCStore) GetItemAsAny(key string) (any, bool) {
	return nil, false
}

func (s *RPCStore) SetItem(key string, v any, sync bool) error {
	// zlog.Info("RPCStore SetItem:", key)
	var item zdict.Item
	item.Name = key
	item.Value = v
	go func() {
		err := zrpc.MainClient.Call("KeyValueRPCCalls.SetItem", item, nil)
		zlog.OnError(err)
	}()
	return nil
}

func (s *RPCStore) RemoveForKey(key string, sync bool) {

}
