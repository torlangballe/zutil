package zkeyvalue

import "github.com/torlangballe/zutil/zslice"

type Option[V any] struct {
	Key         string
	Default     V
	MakeDefault func() V
	value       V
	read        bool
}

var changeHandlers []changeHandler

func (o *Option[V]) Get() V {
	if !o.read {
		if !DefaultStore.GetItem(o.Key, &o.value) {
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
	DefaultStore.SetItem(o.Key, o.value, true)
	for _, h := range changeHandlers {
		h.handler(o.Key)
	}
}

func SetOptionChangeHandler(identifier any, handler func(optionKey string)) {
	if handler != nil {
		changeHandlers = append(changeHandlers, changeHandler{identifier, handler})
		return
	}
	for i := 0; i < len(changeHandlers); i++ {
		if changeHandlers[i].identifier == identifier {
			zslice.RemoveAt(&changeHandlers, i)
			i--
		}
	}
}

type changeHandler struct {
	identifier any
	handler    func(optionKey string)
}
