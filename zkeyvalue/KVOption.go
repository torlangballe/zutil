package zkeyvalue

import (
	"github.com/torlangballe/zutil/zslice"
)

type Option[V any] struct {
	Key         string
	Default     V
	MakeDefault func() V

	value          V
	read           bool
	store          Storer
	changeHandlers []changeHandler
}

func NewOption[V any](store Storer, key string, val V) *Option[V] {
	o := &Option[V]{}
	o.Key = key
	o.value = val
	o.store = store
	return o
}

func (o *Option[V]) Get() V {
	if o.store == nil {
		if o.MakeDefault != nil {
			return o.MakeDefault()
		}
		return o.value
	}
	if !o.read {
		if !o.store.GetItem(o.Key, &o.value) {
			if o.MakeDefault != nil {
				o.value = o.MakeDefault()
			} else {
				o.value = o.Default
			}
		}
		o.read = true
	}
	return o.value
}

func (o *Option[V]) Set(v V) {
	o.value = v
	o.store.SetItem(o.Key, o.value, true)
	for _, h := range o.changeHandlers {
		h.handler()
	}
}

func (o *Option[V]) SetChangedHandler(id any, handler func()) {
	if handler != nil {
		o.changeHandlers = append(o.changeHandlers, changeHandler{id, handler})
		return
	}
	for i := 0; i < len(o.changeHandlers); i++ {
		if o.changeHandlers[i].id == id {
			zslice.RemoveAt(&o.changeHandlers, i)
			i--
		}
	}
}

type changeHandler struct {
	id      any
	handler func()
}
