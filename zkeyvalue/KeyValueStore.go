package zkeyvalue

import (
	"encoding/json"
	"time"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
)

//  Created by Tor Langballe on /30/10/15.

// For storage:
// https://github.com/peterbourgon/diskv
// https://github.com/recoilme/pudge
// https://github.com/nanobox-io/golang-scribble

var (
	DefaultStore        *Store
	DefaultSessionStore *Store

	// IsInTests bool
)

type RawStorer interface {
	RawGetItem(key string, vptr any) bool
	RawGetItemAsAny(key string) (any, bool)
	RawSetItem(key string, v any) error
	RawRemoveForKey(key string) error
	AllKeys() []string
}

type Saver interface {
	Save() error
}

// type SimpleStorer interface {
// 	RawSetItem(key string, v any, sync bool) error
// }

type Store struct {
	Raw   RawStorer
	Saver Saver
	// Secure      bool   // true if key/value stored in secure key chain
	KeyPostfix string // this can be a user id. Not used if key starts with /
	// Path       string // Some variants of store use this
}

var (
	GlobalKeyPostfix string // this is added to ALL key prefixes
)

func init() {
	zdebug.KeyValueSetObjectFunc = func(key string, o any) {
		if DefaultStore != nil {
			DefaultStore.SetObject(o, key, true)
		}
	}
	zdebug.KeyValueGetAndDeleteErrorFunc = func(key string) error {
		if DefaultStore != nil {
			var ce zerrors.ContextError
			if DefaultStore.GetObject(key, &ce) {
				DefaultStore.RemoveForKey(key, true)
				return ce
			}
		}
		return nil
	}
}

func (s Store) GetObject(key string, objectPtr interface{}) (got bool) {
	var rawjson string
	s.postfixKey(&key)
	got = s.Raw.RawGetItem(key, &rawjson)
	if got {
		err := json.Unmarshal([]byte(rawjson), objectPtr)
		if zlog.OnError(err, "unmarshal", string(rawjson)) {
			return
		}
	}
	return
}

func (s Store) GetString(key string) (str string, got bool) {
	s.postfixKey(&key)
	got = s.Raw.RawGetItem(key, &str)
	return
}

func (s Store) GetDict(key string) (dict zdict.Dict, got bool) {
	got = s.GetObject(key, &dict)
	return
}

func (s Store) GetInt64(key string, def int64) (val int64, got bool) {
	s.postfixKey(&key)
	a, got := s.Raw.RawGetItemAsAny(key)
	if got {
		n, err := zint.GetAny(a)
		if zlog.OnError(err) {
			return def, false
		}
		return n, true
	}
	return def, false
}

func (s Store) GetInt(key string, def int) (int, bool) {
	n, got := s.GetInt64(key, int64(def))
	// zlog.Info("KVS GetInt:", key, n, got)
	return int(n), got
}

func (s Store) GetDouble(key string, def float64) (val float64, got bool) {
	s.postfixKey(&key)
	a, got := s.Raw.RawGetItemAsAny(key)
	if got {
		n, err := zfloat.GetAny(a)
		if zlog.OnError(err) {
			return def, false
		}
		return n, true
	}
	return def, false
}

func (s Store) GetTime(key string) (time.Time, bool) {
	return time.Time{}, false
}

func (s Store) GetBool(key string, def bool) (val bool, got bool) {
	s.postfixKey(&key)
	got = s.Raw.RawGetItem(key, &val)
	if got {
		return val, true
	}
	return def, true
}

func (s Store) GetBoolInd(key string, def zbool.BoolInd) (val zbool.BoolInd, got bool) {
	s.postfixKey(&key)
	got = s.Raw.RawGetItem(key, &val)
	if got {
		return val, true
	}
	return def, true
}

func (s Store) IncrementInt(key string, sync bool, inc int) int {
	return 0
}

func (s Store) SetObject(object interface{}, key string, sync bool) {
	data, err := json.Marshal(object)
	if zlog.OnError(err, "marshal") {
		return
	}
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, string(data))
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetString(value string, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetDict(dict zdict.Dict, key string, sync bool) {
	s.SetObject(dict, key, sync)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetInt64(value int64, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetInt(value int, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetDouble(value float64, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetBool(value bool, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetTime(value time.Time, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) SetBoolInd(value zbool.BoolInd, key string, sync bool) {
	s.postfixKey(&key)
	s.Raw.RawSetItem(key, value)
	if sync && s.Saver != nil {
		s.Saver.Save()
	}
}

func (s Store) GetItem(key string, v any) bool {
	s.postfixKey(&key)
	got := s.Raw.RawGetItem(key, v)
	// zlog.Info("S.GetItem", key, reflect.ValueOf(v).Elem())
	return got
}

func (s Store) GetItemAsAny(key string) (any, bool) {
	s.postfixKey(&key)
	return s.Raw.RawGetItemAsAny(key)
}

func (s Store) SetItem(key string, v any, sync bool) error {
	s.postfixKey(&key)
	err := s.Raw.RawSetItem(key, v)
	if err != nil && sync {
		s.Saver.Save()
	}
	return err
}

func (s Store) RemoveForKey(key string, sync bool) error {
	err := s.Raw.RawRemoveForKey(key)
	if err != nil && sync {
		s.Saver.Save()
	}
	return err
}

func (s Store) postfixKey(key *string) {
	if (*key)[0] != '/' && s.KeyPostfix != "" {
		*key = *key + s.KeyPostfix
	}
	if zdebug.IsInTests {
		*key += "_test"
	}
	*key = *key + GlobalKeyPostfix
}

func (s Store) GetAllForPrefix(prefix string) zdict.Dict {
	d := zdict.Dict{}
	for _, key := range s.Raw.AllKeys() {
		a, got := s.GetItemAsAny(key)
		if got {
			d[key] = a
		}
	}
	return d
}
