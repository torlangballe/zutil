package zeventdb

import (
	"testing"

	"github.com/torlangballe/zutil/zlog"
)

func openDB(t *testing.T) *EventDB {
	db := New("/tmp/eventdb")
	return db
}

func testAdd(t *testing.T) {
	zlog.Warn("testAdd")

	edb, err := openDB(t)
	if err != nil {
		t.Error(err, "open db")
		return
	}
}

func TestAll(t *testing.T) {
	testAdd(t)
}
