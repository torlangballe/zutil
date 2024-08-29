package zeventdb

import (
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/ztesting"
)

type Event struct {
	ID     int64
	Time   time.Time
	Type   int32
	TestID int64
}

var (
	types   = []int32{1, 2, 4, 8, 16, 32, 64}
	testIDs = []int64{54656, 3434, 4546, 1232, 8565, 7676}
)

func (e *Event) GetEventDBRow() Row {
	return Row{ID: e.ID, Time: e.Time.UnixMicro(), Type: e.Type, Attribute: e.TestID}
}

func (e *Event) SetInt64ID(id int64) {
	e.ID = id
}

func openDB(t *testing.T) *EventDB {
	db, err := New("/tmp/eventdb")
	if err != nil {
		t.Error("New():", err)
		return nil
	}
	err = db.Load()
	if err != nil {
		t.Error("Load:", err)
		return nil
	}
	return db
}

func makeEvent() Event {
	return Event{
		Time:   time.Now(),
		Type:   *zslice.Random(types),
		TestID: *zslice.Random(testIDs),
	}
}

var edb *EventDB

func testAdd(t *testing.T) {
	zlog.Warn("testAdd")

	edb = openDB(t)
	if edb == nil {
		return
	}
	e := makeEvent()
	id, err := edb.Add(&e)
	if err != nil {
		t.Error(err)
		return
	}
	ztesting.Compare(t, "id same as added id", id, e.ID)
}

func testIterate(t *testing.T) {
	
}

func TestAll(t *testing.T) {
	testAdd(t)
	testIterate(t)
}
