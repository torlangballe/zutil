package zerrors

import (
	"errors"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type ContextError struct {
	Title     string
	SubError  *ContextError
	KeyValues zdict.Dict
}

func (e ContextError) Error() string {
	str := e.Title
	if e.SubError != nil {
		str = zstr.Concat(" / ", str, e.SubError.Error())
	}
	return str
}

func (e ContextError) GetTitle() string {
	return e.Title
}

func (e ContextError) Unwrap() error {
	if e.SubError == nil {
		return nil
	}
	return errors.New(e.SubError.Title)
}

func MakeContextError(dict zdict.Dict, parts ...any) ContextError {
	var ie ContextError
	var nparts []any
	var hasSub bool
	var subError error
	for _, p := range parts {
		e, got := p.(ContextError)
		if got {
			zlog.ErrorIf(hasSub, p, "multiple sub-items-errors")
			copy := e
			ie.SubError = &copy
			hasSub = true
			continue
		}
		err, got := p.(error)
		if got {
			subError = err
			continue
		}
		nparts = append(nparts, p)
	}
	ie.KeyValues = dict
	if !hasSub && subError != nil {
		var ce ContextError
		if errors.As(subError, &ce) {
			if len(nparts) == 0 {
				ce.KeyValues.MergeIn(dict)
				return ce
			}
			ie.SubError = &ce
		} else {
			estr := subError.Error()
			if len(nparts) == 0 {
				ie.Title = subError.Error()
				return ie
			}
			ie.SubError = &ContextError{Title: estr}
		}
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
		c.SubError = &ce
		c.Title = err.Error()
		zstr.HasSuffix(c.Title, " / "+ce.Error(), &c.Title)
		return c, true
	}
	return ContextError{}, false
}
