package zkeyvalue

import (
	"strconv"
	"syscall/js"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
)

func (k Store) getLocalStorage() js.Value {
	if k.SessionOnly {
		return js.Global().Get("sessionStorage")
	}
	return js.Global().Get("localStorage")
}

func (s Store) GetItemAsAny(key string) (any, bool) {
	var i int64
	var f float64
	var b bool
	var str string
	got := s.GetItem(key, &i)
	if got {
		return i, true
	}
	got = s.GetItem(key, &f)
	if got {
		return f, true
	}
	got = s.GetItem(key, &b)
	if got {
		return b, true
	}
	got = s.GetItem(key, &str)
	if got {
		return str, true
	}
	return nil, false
}

func (s Store) GetItem(key string, v interface{}) bool {
	var err error
	s.postfixKey(&key)
	local := s.getLocalStorage()
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
	k.postfixKey(&key)
	local := k.getLocalStorage()
	local.Set(key, v)
	return nil
}

func (k Store) RemoveForKey(key string, sync bool) {
	k.postfixKey(&key)
	k.getLocalStorage().Call("removeItem", key)
}
