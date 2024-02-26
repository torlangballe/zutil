package zkeyvalue

import (
	"reflect"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslice"
)

type Option[V comparable] struct {
	Key     string
	Default V
	value   V
	store   *Store
	gotten  bool
}

var optionChangedHandlers []optionChangeHandler

func NewOption[V comparable](store *Store, key string, val V) *Option[V] {
	o := &Option[V]{}
	o.Key = key
	o.value = val
	o.store = store
	return o
}

func (o *Option[V]) Get() V {
	if o.gotten {
		return o.value
	}
	if o.store == nil {
		o.store = DefaultStore
	}
	if o.store.GetItem(o.Key, &o.value) {
		o.gotten = true
		return o.value
	}
	o.gotten = true
	return o.value
}

func (o *Option[V]) Set(v V, callHandle bool) {
	// zlog.Info("O.Set:", o != nil)
	if o.value == v {
		return
	}
	if o.store == nil {
		o.store = DefaultStore
	}
	o.value = v
	err := o.store.SetItem(o.Key, o.value, true)
	zlog.OnError(err)
	if !callHandle {
		return
	}
	for _, h := range optionChangedHandlers {
		if h.key == o.Key {
			h.handler()
		}
	}
}

func (o *Option[V]) SetAny(a any, callHandle bool) {
	var v V
	pval := reflect.ValueOf(&v)
	aval := reflect.ValueOf(a)
	kind := zreflect.KindFromReflectKindAndType(aval.Kind(), aval.Type())
	switch kind {
	case zreflect.KindInt, zreflect.KindFloat:
		if kind == zreflect.KindFloat {
			zfloat.SetAny(pval.Interface(), aval.Float())
		} else {
			zint.SetAny(pval.Interface(), aval.Int())
		}

	case zreflect.KindString:
		// zlog.Info("SetString?:", pval.Type(), pval.Kind(), pval.String(), o.Key)
		sptr := pval.Interface().(*string)
		*sptr = aval.String()

	case zreflect.KindBool:
		bptr := pval.Interface().(*bool)
		*bptr = aval.Bool()
	}
	o.Set(v, callHandle)
}

func (o *Option[V]) AddChangedHandler(handler func()) {
	AddOptionChangedHandler(o, o.Key, handler)
}

func AddOptionChangedHandler(id any, key string, handler func()) {
	if handler != nil {
		h := optionChangeHandler{
			id:      id,
			key:     key,
			handler: handler,
		}
		optionChangedHandlers = append(optionChangedHandlers, h)
		return
	}
}

func removeChangedHandler(id any, key string) {
	for i := 0; i < len(optionChangedHandlers); i++ {
		h := &optionChangedHandlers[i]
		if h.id == id && (key == "" || h.key == key) {
			zslice.RemoveAt(&optionChangedHandlers, i)
			i--
		}
	}
}

func findHandler(id any, key string) (*optionChangeHandler, int) {
	for i, h := range optionChangedHandlers {
		if id == h.id && key == "" || key == h.key {
			return &optionChangedHandlers[i], -1
		}
	}
	return nil, -1
}

type optionChangeHandler struct {
	id      any
	key     string
	handler func()
}
