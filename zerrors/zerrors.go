package zerrors

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zstr"
)

type ContextError struct {
	Title           string        `json:",omitempty"`
	SubContextError *ContextError `json:",omitempty"`
	WrappedError    error         `json:"-"`
	KeyValues       zdict.Dict    `json:",omitempty"`
}

func (e ContextError) Error() string {
	var add string
	if e.WrappedError != nil {
		add = e.WrappedError.Error()
	}
	return zstr.Concat(": ", e.Title, add)
}

func (e ContextError) GetTitle() string {
	return e.Title
}

func (e ContextError) String() string {
	str := e.Title
	if e.SubContextError != nil {
		str += " { " + e.SubContextError.String() + " } "
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

func (e *ContextError) GetLowerCaseMatchContent() string {
	str := e.Title
	for k, v := range e.KeyValues {
		str += "," + k + "," + fmt.Sprint(v)
	}
	str = strings.ToLower(str)
	if e.SubContextError != nil {
		str += "," + e.SubContextError.GetLowerCaseMatchContent()
	}
	return str
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

func (ce *ContextError) UnmarshalJSON(data []byte) error {
	type ce2 ContextError
	err := json.Unmarshal(data, (*ce2)(ce))
	if err != nil {
		return err
	}
	for k, v := range ce.KeyValues {
		if strings.HasSuffix(k, "Link") {
			str := fmt.Sprint(v)
			if strings.HasPrefix(str, "http://") || strings.HasPrefix(str, "https://") {
				ce.KeyValues[k] = zstr.URLWrapper(str)
			}
			continue
		}
		if k == "Code File" {
			var sname, sline string
			str := reflect.ValueOf(v).String()
			if zstr.SplitN(str, ":", &sname, &sline) && sname != "" && sline != "" {
				_, err := strconv.Atoi(sline)
				if err == nil {
					codeLink := zstr.CodeLink(str)
					ce.KeyValues[k] = codeLink
				}
			}
		}
	}
	return nil
}
