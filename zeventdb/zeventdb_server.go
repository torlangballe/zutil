//go:build server

package zeventdb

import (
	"encoding/json"
	"sync"

	"github.com/lotusdblabs/lotusdb/v2"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
)

type Row struct {
	ID        int64
	Time      int64
	Type      int32
	Attribute int64
}

type RowGetter interface {
	GetEventDBRow() Row
}

type IDSetter interface {
	SetInt64ID(id int64)
}

// type SearchStringGetter interface {
// 	GetSearchString() string
// }

type EventDB struct {
	IDIndex int64
	Rows    *zslice.LinkedSlice[Row, int64]
	DB      *lotusdb.DB

	lock sync.Mutex
}

func New(dbFile string) (*EventDB, error) {
	var err error
	var got bool
	edb := &EventDB{}
	lopts := lotusdb.DefaultOptions
	lopts.DirPath = dbFile
	edb.DB, err = lotusdb.Open(lopts)
	zlog.Assert(err == nil, dbFile)

	opts := zslice.DefaultLSOpts
	opts.DB = edb.DB
	opts.DelaySortAddSecs = 1
	opts.SaveSecs = 5
	edb.Rows = zslice.NewLinked[Row, int64](opts, func(r Row) int64 {
		return r.Time
	})
	edb.Rows.GetIDFunc = func(r Row) int64 {
		return r.ID
	}
	edb.IDIndex, got, err = edb.Rows.GetInt64ForKeyFromDB(idIndexKey)
	if err != nil {
		return nil, err
	}
	if !got {
		edb.IDIndex = 1
	}
	return edb, nil
}

func (e *EventDB) GetIncIDIndex() int64 {
	index := e.IDIndex
	e.IDIndex++
	e.Rows.SetInt64ForKeyToDB(idIndexKey, e.IDIndex)
	return index
}

func (e *EventDB) Load() error {
	return e.Rows.Load()
}

func (e *EventDB) Add(event RowGetter) (int64, error) {
	e.lock.Lock()
	defer e.lock.Unlock()
	row := event.GetEventDBRow()
	row.ID = e.GetIncIDIndex()
	err := e.Rows.SetInt64ForKeyToDB(idIndexKey, e.IDIndex)
	if zlog.OnError(err) {
		return 0, err
	}

	iset, is := event.(IDSetter)
	if is {
		iset.SetInt64ID(row.ID)
	}
	jdata, err := json.Marshal(event)
	if zlog.OnError(err) {
		return 0, err
	}
	zlog.Warn("AddEvent:", row.ID, string(jdata))
	return row.ID, nil
}

const (
	chunkSize  = 200
	idIndexKey = "eidx"
)

type Iterator struct {
	Time      int64
	TypeMask  int32
	Attribute int64
	Forward   bool
}

func (e *EventDB) Iterate(iter Iterator, dataPtr any, give func(row *Row) bool) {
	e.Rows.BottomLock.Lock()
	defer e.Rows.BottomLock.Lock()

	rowsLen := e.Rows.Len()
	var si int
	if iter.Time == 0 {
		if iter.Forward {
			si = 0
		} else {
			si = rowsLen - 1
		}
	} else {
		i, _ := e.Rows.BinarySearch(iter.Time)
		if iter.Forward {
			si = i + 1
		} else {
			si = i - 1
		}
	}
	end := 0
	inc := -1
	if iter.Forward {
		end = rowsLen - 1
		inc = 1
	}
	for i := si; i != end; i += inc {
		row := e.Rows.Index(i)
		if iter.TypeMask != 0 && row.Type&iter.TypeMask == 0 {
			continue
		}
		if iter.Attribute != 0 && row.Attribute != iter.Attribute {
			continue
		}
		//		if iter.TextMatchLowerCase != "" {}
		if !give(row) {
			break
		}
	}
}

func (e *EventDB) GetAuxData(id int64, dataPtr any) error {
	return e.Rows.GetAuxData(id, dataPtr)
}
