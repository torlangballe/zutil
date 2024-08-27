package zeventdb

import (
	"github.com/lotusdblabs/lotusdb/v2"
	"github.com/torlangballe/zutil/zslice"
)

const chunkSize = 200

type Row struct {
	ID     int64
	Time   int64
	Type   int64
	TestID int64
}

type DB struct {
	IDIndex int64
	Rows    *zslice.LinkedSlice[Row]
	DB      *lotusdb.DB
}
