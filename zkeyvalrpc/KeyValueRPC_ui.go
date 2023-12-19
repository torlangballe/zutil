//go:build zui

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/ztimer"
)

type RPCStore struct {
	zkeyvalue.Store
}

var rpcStore = newRPCStore()

func newRPCStore() *RPCStore {
	s := &RPCStore{}
	drs := zkeyvalue.NewDictRawStore()
	s.Store.Raw = drs
	var getDict zdict.Dict
	ztimer.StartIn(0.1, func() { // make sure MainClient is set up
		err := zrpc.MainClient.Call("KeyValueRPCCalls.ReadStore", nil, &getDict)
		if zlog.OnError(err) {
			return
		}
		drs.Set(getDict)
	})
	return s
}

func NewOption[V any](key string, val V) *zkeyvalue.Option[V] {
	o := zkeyvalue.NewOption[V](rpcStore, key, val)
	o.SetChangedHandler(rpcStore, func() {
		rpcStore.SetItem(o.Key, o.Get(), true)
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
