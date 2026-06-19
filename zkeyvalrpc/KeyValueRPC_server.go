//go:build server

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/xrpc"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/znamedfuncs"
)

type KeyValueRPCCalls struct{}

var (
	storePath string
	rpcStore  *zkeyvalue.Store
)

func Init(storePath string) {
	rpcStore = zkeyvalue.NewFileStore(storePath)
	for _, key := range rpcStore.Raw.AllKeys() {
		f, gotHandler := externalChangeHandlers.Get(key)
		if gotHandler {
			val, got := rpcStore.GetItemAsAny(key)
			if got {
				f(key, val, false)
			}
		}
	}
}

func NewOption[V comparable](key string, val V) *zkeyvalue.Option[V] {
	o := zkeyvalue.NewOption[V](&rpcStore, key, val)
	AddExternalChangedHandler(key, func(key string, value any, isLoad bool) {
		o.SetAny(value, true)
	})
	o.AddChangedHandler(func() {
		xrpc.SetResourceUpdated(ResourceID, "")
	})
	return o
}

func (KeyValueRPCCalls) GetAll(in struct{}, store *zdict.Dict) error {
	d := zdict.Dict{}
	for _, key := range rpcStore.Raw.AllKeys() {
		val, got := rpcStore.GetItemAsAny(key)
		if got {
			d[key] = val
		}
	}
	*store = d
	return nil
}

func (KeyValueRPCCalls) SetItem(ci *znamedfuncs.ClientInfo, kv zdict.Item) error {
	rpcStore.SetItem(kv.Name, kv.Value, true)
	f, got := externalChangeHandlers.Get(kv.Name)
	if got {
		f(kv.Name, kv.Value, true)
	}
	err := rpcStore.Saver.Save()
	return err
}
