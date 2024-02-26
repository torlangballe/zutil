//go:build !js

package zcache

import (
	"os"
	"strings"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

func (m *ExpiringMap[K, V]) SetStorage(fpath string) {
	if !strings.HasPrefix(fpath, "/") {
		fpath = zfile.JoinPathParts(os.TempDir(), fpath)
	}
	fpath = zfile.ChangedExtension(fpath, ".json")
	m.storagePath = fpath
	d := map[K]V{}
	err := zjson.UnmarshalFromFile(&d, fpath, true)
	if zlog.OnError(err) {
		return
	}
	for k, v := range d {
		m.Set(k, v)
	}
	ztimer.RepeatForever(5, func() {
		m.FlushToStorage()
	})
}

func (m *ExpiringMap[K, V]) FlushToStorage() {
	if m.changed {
		d := map[K]V{}
		m.ForAll(func(key K, value V) {
			d[key] = value
		})
		err := zjson.MarshalToFile(d, m.storagePath)
		if zlog.OnError(err) {
			return
		}
		m.changed = false
	}
}
