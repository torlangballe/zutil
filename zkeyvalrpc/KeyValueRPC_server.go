//go:build server

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zrpc"
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
		zrpc.SetResourceUpdated(ResourceID, "")
	})
	return o
}

func (KeyValueRPCCalls) GetAll(in *zrpc.Unused, store *zdict.Dict) error {
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

func (KeyValueRPCCalls) SetItem(ci *zrpc.ClientInfo, kv zdict.Item, result *zrpc.Unused) error {
	// zlog.Info("zkeyvalrpc SetItem1:", kv)
	rpcStore.SetItem(kv.Name, kv.Value, true)
	f, got := externalChangeHandlers.Get(kv.Name)
	if got {
		f(kv.Name, kv.Value, true)
	}
	// zlog.Info("zkeyvalrpc SetItem:", rpcStore.DictRawStore().All())
	err := rpcStore.Saver.Save()
	return err
}
