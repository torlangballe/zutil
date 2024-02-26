package zkeyvalue

import (
	"reflect"
	"sync"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zreflect"
)

type DictRawStore struct {
	lock sync.Mutex
	dict zdict.Dict
}

func NewDictRawStore() *DictRawStore {
	d := &DictRawStore{}
	d.dict = zdict.Dict{}
	return d
}

func (d *DictRawStore) AllKeys() []string {
	d.lock.Lock()
	keys := d.dict.Keys()
	d.lock.Unlock()
	return keys
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

func (s *DictRawStore) RawSetItem(key string, v any) error {
	s.lock.Lock()
	s.dict[key] = v
	s.lock.Unlock()
	return nil
}

func (s *DictRawStore) RawRemoveForKey(key string) error {
	s.lock.Lock()
	delete(s.dict, key)
	s.lock.Unlock()
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
