//go:build !js

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zrpc"
)

type KeyValueRPCCalls struct{}

var (
	storePath string
	rpcStore  = zkeyvalue.NewFileStore("")
)

func Init(storePath string) {
	rpcStore.Load(storePath)
	for key, val := range rpcStore.DictRawStore().All() {
		// zlog.Info("zkeyvalrpc load:", key, val)
		f, got := externalChangeHandlers.Get(key)
		if got {
			f(key, val, false)
		}
	}
}

func NewOption[V comparable](key string, val V) *zkeyvalue.Option[V] {
	o := zkeyvalue.NewOption[V](rpcStore, key, val)
	AddExternalChangedHandler(key, func(key string, value any, isLoad bool) {
		// zlog.Info("zkeyvalrpc ExtHandler:", key)
		o.SetAny(value, true)
	})
	o.AddChangedHandler(func() {
		// zlog.Info("keyvalrpc changed:", key)
		zrpc.SetResourceUpdated(ResourceID, "")
	})
	return o
}

func (KeyValueRPCCalls) GetAll(in *zrpc.Unused, store *zdict.Dict) error {
	drs := rpcStore.DictRawStore()
	*store = drs.All()
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
	rpcStore.Save()
	return nil
}
