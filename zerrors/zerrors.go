package zerrors

import (
	"errors"
	"fmt"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zstr"
)

type ContextError struct {
	Title           string
	SubContextError *ContextError
	WrappedError    error `json:"-"`
	KeyValues       zdict.Dict
}

// func init() {
// 	zlog.MakeContextErrorFunc = func(m map[string]any, parts ...any) error {
// 		dict := zdict.FromShallowMap(m)
// 		for i, part := range parts {
// 			spart := fmt.Sprint(part)
// 			str := zstr.ColorSetter.Replace(spart)
// 			if str != spart {
// 				parts[i] = str
// 				parts = append(parts, zstr.EscNoColor)
// 			}
// 		}
// 		ce := MakeContextError(dict, parts...)
// 		return ce
// 	}
// }

func (e ContextError) Error() string {
	str := e.Title
	if e.WrappedError != nil {
		str += ": " + e.WrappedError.Error()
	}
	return str
	// str := e.Title
	// if e.SubContextError != nil {
	// 	str = zstr.Concat(" / ", str, e.SubContextError.Error())
	// } else if e.WrappedError != nil {
	// 	str = zstr.Concat(" / ", str, e.WrappedError.Error())
	// }
	// return str
}

func (e ContextError) GetTitle() string {
	return e.Title
}

func (e ContextError) String() string {
	str := fmt.Sprintf("{ %s %+v ", e.Title, e.KeyValues)
	if e.SubContextError != nil {
		str += "{ " + e.SubContextError.String() + " } "
	}
	return str + "}"
}

func (e ContextError) Unwrap() error {
	if e.SubContextError != nil {
		return *e.SubContextError
	}
	return e.WrappedError
}

func (e *ContextError) AddSub(dict zdict.Dict, parts ...any) {
	sub := MakeContextError(dict, parts...)
	e.SubContextError = &sub
}

func (e *ContextError) SetError(err error) {
	if e.KeyValues == nil {
		e.KeyValues = zdict.Dict{}
	}
	e.KeyValues["Error"] = err.Error()
}

func (e *ContextError) SetKeyValue(key string, value any) {
	if e.KeyValues == nil {
		e.KeyValues = zdict.Dict{}
	}
	e.KeyValues[key] = value
}

func MakeContextError(dict zdict.Dict, parts ...any) ContextError {
	var ie ContextError
	var nparts []any
	ie.KeyValues = dict
	for _, p := range parts {
		err, got := p.(error)
		if got {
			ie.WrappedError = err
			ce, gotCE := ContextErrorFromError(err)
			if gotCE {
				if ie.SubContextError != nil {
					fmt.Println("MakeContextError: assert ie.SubContextError != nil", p, "multiple sub-items-errors")
				}
				ie.SubContextError = &ce
				continue
			}
			ie.SetError(err)
			continue
		}
		nparts = append(nparts, p)
	}
	ie.Title = zstr.Spaced(nparts...)
	return ie
}

func ContextErrorFromError(err error) (ContextError, bool) {
	ce, got := err.(ContextError)
	if got {
		return ce, true
	}
	if errors.As(err, &ce) {
		var c ContextError
		c.SubContextError = &ce
		c.Title = err.Error()
		zstr.HasSuffix(c.Title, " / "+ce.Error(), &c.Title)
		return c, true
	}
	return ContextError{}, false
}
