package zkeyvalue

import (
	"reflect"
	"sync"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zreflect"
)

type DictRawStore struct {
	lock  sync.Mutex
	dict  zdict.Dict
	Saver Saver
}

func NewDictRawStore() *DictRawStore {
	d := &DictRawStore{}
	d.dict = zdict.Dict{}
	return d
}

func (s *DictRawStore) RawGetItem(key string, pointer any) bool {
	gval, got := s.RawGetItemAsAny(key)
	if got {
		rval := reflect.ValueOf(pointer).Elem()
		zreflect.AnySetWithRelaxedNumbers(rval, reflect.ValueOf(gval))
		return true
	}
	return false
}

func (s *DictRawStore) RawGetItemAsAny(key string) (any, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	gval, got := s.dict[key]
	return gval, got
}

func (s *DictRawStore) RawSetItem(key string, v any, sync bool) error {
	s.lock.Lock()
	s.dict[key] = v
	s.lock.Unlock()
	if sync && s.Saver != nil {
		return s.Saver.Save()
	}
	return nil
}

func (s *DictRawStore) RawRemoveForKey(key string, sync bool) error {
	s.lock.Lock()
	delete(s.dict, key)
	s.lock.Unlock()
	if sync && s.Saver != nil {
		return s.Saver.Save()
	}
	return nil
}

func (s *DictRawStore) Set(dict zdict.Dict) {
	s.dict = dict
}

func (s *DictRawStore) All() zdict.Dict {
	s.lock.Lock()
	d := s.dict.Copy()
	s.lock.Unlock()
	return d
}
