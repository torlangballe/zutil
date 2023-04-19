package zkeyvalue

import (
	"encoding/json"
	"time"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
)

//  Created by Tor Langballe on /30/10/15.

// For storage:
// https://github.com/peterbourgon/diskv
// https://github.com/recoilme/pudge
// https://github.com/nanobox-io/golang-scribble

type Store struct {
	Local     bool   // if true, only for single browser or device, otherwise for user anywhere
	Secure    bool   // true if key/value stored in secure key chain
	KeyPrefix string // this can be a user id. Not used if key starts with /
	filepath  string // Some variants of store use this
}

var DefaultStore *Store

func NewStore(local bool) *Store {
	return &Store{Local: local}
}

func (s Store) GetObject(key string, objectPtr interface{}) (got bool) {
	var rawjson string
	got = s.GetItem(key, &rawjson)
	if got {
		err := json.Unmarshal([]byte(rawjson), objectPtr)
		if zlog.OnError(err, "unmarshal") {
			return
		}
	}
	return
}

func (s Store) GetString(key string) (str string, got bool) {
	got = s.GetItem(key, &str)
	return
}

func (s Store) GetDict(key string) (dict zdict.Dict, got bool) {
	got = s.GetObject(key, &dict)
	return
}

func (s Store) GetInt64(key string, def int64) (val int64, got bool) {
	got = s.GetItem(key, &val)
	if got {
		return
	}
	return def, true
}

func (s Store) GetInt(key string, def int) (int, bool) {
	n, got := s.GetInt64(key, int64(def))
	// zlog.Info("KVS GetInt:", key, n, got)
	return int(n), got
}

func (s Store) GetDouble(key string, def float64) (val float64, got bool) {
	got = s.GetItem(key, &val)
	if got {
		return val, true
	}
	return def, true
}

func (s Store) GetTime(key string) (time.Time, bool) {
	return time.Time{}, false
}

func (s Store) GetBool(key string, def bool) (val bool, got bool) {
	got = s.GetItem(key, &val)
	if got {
		return val, true
	}
	return def, true
}

func (s Store) GetBoolInd(key string, def zbool.BoolInd) (val zbool.BoolInd, got bool) {
	got = s.GetItem(key, &val)
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
	s.SetItem(key, string(data), sync)
}
func (s Store) SetString(value string, key string, sync bool)  { s.SetItem(key, value, sync) }
func (s Store) SetDict(dict zdict.Dict, key string, sync bool) { s.SetObject(dict, key, sync) }
func (s Store) SetInt64(value int64, key string, sync bool)    { s.SetItem(key, value, sync) }
func (s Store) SetInt(value int, key string, sync bool)        { s.SetItem(key, value, sync) }
func (s Store) SetDouble(value float64, key string, sync bool) { s.SetItem(key, value, sync) }
func (s Store) SetBool(value bool, key string, sync bool)      { s.SetItem(key, value, sync) }
func (s Store) SetTime(value time.Time, key string, sync bool) { s.SetItem(key, value, sync) }
func (s Store) ForAllKeys(got func(key string))                {}
func (s Store) SetBoolInd(value zbool.BoolInd, key string, sync bool) {
	s.SetItem(key, int(value), sync)
}

func (s Store) prefixKey(key *string) {
	if (*key)[0] != '/' && s.KeyPrefix != "" {
		*key = s.KeyPrefix + "/" + *key
	}
	if zlog.IsInTests {
		*key += "_test"
	}
}
