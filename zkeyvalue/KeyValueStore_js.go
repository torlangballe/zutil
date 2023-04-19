package zkeyvalue

import (
	"strconv"
	"syscall/js"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
)

func getLocalStorage() js.Value {
	return js.Global().Get("localStorage")
}

func (k Store) GetItem(key string, v interface{}) bool {
	var err error
	k.prefixKey(&key)
	local := getLocalStorage()
	o := local.Get(key)

	// zlog.Info("get kv item:", key, o.Type(), o)
	switch o.Type() {
	case js.TypeUndefined:
		// zlog.Debug(nil, zlog.StackAdjust(1), "Store GetItem item undefined:", key)
		return false

	case js.TypeNumber:
		zfloat.SetAny(v, o.Float())
		return true

	case js.TypeBoolean:
		*v.(*bool) = o.Bool()
		return true

	case js.TypeString:
		sp, _ := v.(*string)
		if sp != nil {
			*sp = o.String()
			// zlog.Info("get kv item string:", o.String())
		}
		ib, _ := v.(*bool)
		if ib != nil {
			*ib, err = zbool.FromStringWithError(o.String())
			if zlog.OnError(err, "parse bool") {
				return false
			}
		}
		ip, _ := v.(*int64)
		if ip != nil {
			*ip, err = strconv.ParseInt(o.String(), 10, 64)
			if zlog.OnError(err, "parse int") {
				return false
			}
			// zlog.Info("get kv item int:", *ip)
		}
		fp, _ := v.(*float64)
		if fp != nil {
			*fp, err = strconv.ParseFloat(o.String(), 64)
			if zlog.OnError(err, "parse float") {
				return false
			}
			// zlog.Info("get kv item float:", o.Float())
		}
		return true
	}
	zlog.Debug("bad type:", o.Type())
	return false
}

func (k *Store) SetItem(key string, v interface{}, sync bool) error {
	k.prefixKey(&key)
	local := getLocalStorage()
	local.Set(key, v)
	return nil
}

func (s Store) RemoveForKey(key string, sync bool) {
	s.prefixKey(&key)
	getLocalStorage().Call("removeItem", key)
}
