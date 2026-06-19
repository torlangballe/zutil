//go:build zui

package zkeyvalrpc

import (
	"github.com/torlangballe/zutil/xrpc"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
)

var rpcStore = newRPCStore()

type dictRPC struct {
	zkeyvalue.DictRawStore
}

func Init() {
	// xrpc.MainCaller().RegisterPollGetter(ResourceID, getAll)
	xrpc.RegisterResources(ResourceID)
}

func newRPCStore() *zkeyvalue.Store {
	s := &zkeyvalue.Store{}
	drs := &dictRPC{}
	s.Raw = drs
	s.Saver = drs
	return s
}

func (d *dictRPC) Save() error {
	return nil
}

func (d *dictRPC) RawSetItem(key string, v any) error {
	var item zdict.Item
	item.Name = key
	item.Value = v
	go func() {
		err := xrpc.MainCaller().Call("KeyValueRPCCalls.SetItem", item, nil)
		zlog.OnError(err)
	}()
	return nil
}

func getAll() {
	go func() {
		var dict zdict.Dict
		err := xrpc.MainCaller().Call("KeyValueRPCCalls.GetAll", nil, &dict)
		if zlog.OnError(err) {
			return
		}
		old := rpcStore.Raw.(*dictRPC).All()
		for k, v := range dict {
			oldVal, got := old[k]
			if !got || oldVal != v {
				externalChangeHandlers.ForAll(func(key string, f func(key string, value any, isLoad bool)) {
					f(k, v, true)
				})
				old[k] = v
			}
		}
		drs := rpcStore.Raw.(*dictRPC)
		drs.Set(dict)
	}()
}

func NewOption[V comparable](key string, val V) *zkeyvalue.Option[V] {
	o := zkeyvalue.NewOption[V](&rpcStore, key, val)
	AddExternalChangedHandler(key, func(inkey string, value any, isLoad bool) {
		if inkey == key {
			// zlog.Info("kvrpc got:", inkey, value, o.Get())
			o.SetAny(value, true)
			// zlog.Info("kvrpc got2:", inkey, value, o.Get())
		}
	})
	return o
}
