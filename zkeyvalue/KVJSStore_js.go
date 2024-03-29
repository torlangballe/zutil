package zkeyvalue

import (
	"strconv"
	"syscall/js"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
)

type JSRawStore struct {
	SessionOnly bool // if true, only for while a "session" is open.
}

func init() {
	DefaultStore = NewJSStore(false)
	DefaultSessionStore = NewJSStore(true)
}

func NewJSStore(session bool) *Store {
	s := &Store{}
	var jsRaw JSRawStore
	jsRaw.SessionOnly = session
	s.Raw = &jsRaw
	return s
}

func (k *JSRawStore) getLocalStorage() js.Value {
	if k.SessionOnly {
		return js.Global().Get("sessionStorage")
	}
	return js.Global().Get("localStorage")
}

func (d *JSRawStore) AllKeys() []string {
	zlog.Fatal("Don't call me")
	return nil
}

func (s *JSRawStore) RawGetItemAsAny(key string) (any, bool) {
	var i int64
	var f float64
	var b bool
	var str string
	got := s.RawGetItem(key, &i)
	if got {
		return i, true
	}
	got = s.RawGetItem(key, &f)
	if got {
		return f, true
	}
	got = s.RawGetItem(key, &b)
	if got {
		return b, true
	}
	got = s.RawGetItem(key, &str)
	if got {
		return str, true
	}
	return nil, false
}

func (s *JSRawStore) RawGetItem(key string, v any) bool {
	var err error
	local := s.getLocalStorage()
	o := local.Get(key)

	if o.IsUndefined() {
		return false
	}
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
			if err != nil {
				return false
			}
		}
		ip, _ := v.(*int64)
		if ip != nil {
			*ip, err = strconv.ParseInt(o.String(), 10, 64)
			if err != nil {
				return false
			}
			// zlog.Info("get kv item int:", *ip)
		}
		fp, _ := v.(*float64)
		if fp != nil {
			*fp, err = strconv.ParseFloat(o.String(), 64)
			if err != nil {
				return false
			}
			// zlog.Info("get kv item float:", o.Float())
		}
		return true
	}
	zlog.Debug("bad type:", o.Type())
	return false
}

func (k *JSRawStore) RawSetItem(key string, v any) error {
	local := k.getLocalStorage()
	local.Set(key, v)
	return nil
}

func (k JSRawStore) RawRemoveForKey(key string) error {
	k.getLocalStorage().Call("removeItem", key)
	return nil
}
