//go:build server

package zeventdb

import (
	"encoding/json"

	"github.com/lotusdblabs/lotusdb/v2"
	"github.com/torlangballe/zutil/zbytes"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
)

type Row struct {
	ID         int64
	Time       int64
	Type       int64
	Attribute  int64
	Attribute2 int64
}

type RowGetter interface {
	GetEventDBRow() Row
}

type SearchStringGetter interface {
	GetSearchString() string
}

type EventDB struct {
	IDIndex int64
	Rows    *zslice.LinkedSlice[Row, int64]
	DB      *lotusdb.DB
}

func New(dbFile string) (*EventDB, error) {
	var err error
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
	edb.IDIndex, err = edb.Rows.GetInt64ForKeyFromDB([]byte(iDIndexKey))
	if err != nil {
		return nil, err
	}
	return edb, nil
}

func (e *EventDB) Add(event RowGetter) error {
	row := event.GetEventDBRow()
	row.ID = e.IDIndex
	e.IDIndex++
	err := e.DB.Put([]byte(iDIndexKey), zbytes.NumberToBytes(uint64(e.IDIndex)))
	if zlog.OnError(err) {
		return err
	}
	jdata, err := json.Marshal(event)
	if zlog.OnError(err) {
		return err
	}
	zlog.Warn("AddEvent:", string(jdata))
	return nil
}

const (
	chunkSize  = 200
	iDIndexKey = "eidix"
)
