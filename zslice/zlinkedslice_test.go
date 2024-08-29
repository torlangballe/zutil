//go:build !js && experimental

package zslice

import (
	"fmt"
	"testing"
	"time"

	"github.com/lotusdblabs/lotusdb/v2"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

type LinkedIntSlice = LinkedSlice[int, int]

func checkBinarySearchForChunk(t *testing.T, slice *LinkedIntSlice, val, wantChunkIndex int, before zbool.BoolInd) {
	index, bef := slice.BinarySearchForChunk(val)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearchForChunk %d:", val), index, wantChunkIndex)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearchForChunk %d before:", val), bef, before)
}

func checkBinarySearch(t *testing.T, slice *LinkedIntSlice, val, wantIndex int, exact bool) {
	index, ex := slice.BinarySearch(val)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearch %d:", val), index, wantIndex)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearch %d accurate:", val), ex, exact)
}

func makeSlice(db *lotusdb.DB) *LinkedIntSlice {
	opts := DefaultLSOpts
	opts.ChunkSize = 5
	opts.SaveSecs = 0.1
	opts.DB = db
	return NewLinked[int, int](opts, func(t int) int {
		return t
	})
}

func addStandardNumbers(slice *LinkedIntSlice) {
	for i := 1; i < 35; i += 3 {
		slice.Add(i, nil)
	}
}
func testAdd(t *testing.T) {
	zlog.Warn("zlinkedslice testAdd")
	slice := makeSlice(nil)
	addStandardNumbers(slice)
	// zlog.Warn("Slice:", zlog.Full(*slice))

	ztesting.Compare(t, "Len", slice.Len(), 12)

	checkBinarySearchForChunk(t, slice, 0, 0, zbool.True)
	checkBinarySearchForChunk(t, slice, 1, 0, zbool.Unknown)
	checkBinarySearchForChunk(t, slice, 5, 0, zbool.Unknown)
	checkBinarySearchForChunk(t, slice, 28, 1, zbool.Unknown)
	checkBinarySearchForChunk(t, slice, 34, 2, zbool.Unknown)
	checkBinarySearchForChunk(t, slice, 35, 2, zbool.False)

	checkBinarySearch(t, slice, 0, 0, false)
	checkBinarySearch(t, slice, 1, 0, true)
	checkBinarySearch(t, slice, 4, 1, true)
	checkBinarySearch(t, slice, 7, 2, true)
	checkBinarySearch(t, slice, 13, 4, true)
	checkBinarySearch(t, slice, 14, 5, false)
	checkBinarySearch(t, slice, 15, 5, false)
	checkBinarySearch(t, slice, 16, 5, true)
	checkBinarySearch(t, slice, 17, 6, false)
	checkBinarySearch(t, slice, 19, 6, true)
	checkBinarySearch(t, slice, 28, 9, true)
	checkBinarySearch(t, slice, 31, 10, true)
	checkBinarySearch(t, slice, 34, 11, true)
	checkBinarySearch(t, slice, 35, 12, false)
}

func openDB(t *testing.T) *lotusdb.DB {
	options := lotusdb.DefaultOptions
	options.DirPath = "/tmp/lotusdb_batch"
	db, err := lotusdb.Open(options)
	if err != nil {
		t.Error(err)
		return nil
	}
	return db
}

func testSave(t *testing.T) {
	zlog.Warn("zlinkedslice testSave")

	db := openDB(t)
	if db == nil {
		return
	}
	defer func() {
		err := db.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	slice := makeSlice(db)
	addStandardNumbers(slice)
	slice.Flush()

	time.Sleep(time.Millisecond * 200)

	iter, err := db.NewIterator(lotusdb.IteratorOptions{Reverse: false})
	if err != nil {
		t.Error(err)
		return
	}
	for iter.Valid() {
		// fmt.Println(string(iter.Key()), "data:", iter.Value())
		iter.Next()
	}
	err = iter.Close()
	if err != nil {
		t.Error(err)
		return
	}
}

func testLoad(t *testing.T) {
	db := openDB(t)
	if db == nil {
		return
	}
	defer func() {
		err := db.Close()
		if err != nil {
			t.Error(err)
		}
	}()
	slice := makeSlice(db)
	slice.Load()

	slice2 := makeSlice(nil)
	addStandardNumbers(slice2)

	time.Sleep(time.Millisecond * 200) // loading happens in a go routine and takes amount of time
	ztesting.Compare(t, "loaded len is same", slice.len(), slice2.len())
}

func sliceAsString(slice *LinkedIntSlice) string {
	var str string
	len := slice.Len()
	for i := 0; i < len; i++ {
		if str != "" {
			str += ","
		}
		str += fmt.Sprint(*slice.Index(i))
	}
	return str
}

func testDelayed(t *testing.T) {
	zlog.Warn("zlinkedslice testDelayed")
	opts := DefaultLSOpts
	opts.ChunkSize = 5
	opts.DelaySortAddSecs = 0.05
	slice := NewLinked[int, int](opts, func(t int) int {
		return t
	})
	slice.Add(1, nil)
	slice.Add(2, nil)
	slice.Add(3, nil)
	slice.Add(4, nil)

	time.Sleep(time.Millisecond * 200)
	ztesting.Compare(t, "delayed add1", sliceAsString(slice), "1,2,3,4")

	slice.Add(5, nil)
	slice.Add(7, nil)
	slice.Add(6, nil)
	slice.Add(8, nil)

	time.Sleep(time.Millisecond * 200)
	ztesting.Compare(t, "delayed needs sorting", sliceAsString(slice), "1,2,3,4,5,6,7,8")

	slice.Add(9, nil)
	slice.Add(11, nil)
	time.Sleep(time.Millisecond * 200)
	slice.Add(10, nil)
	slice.Add(12, nil)

	time.Sleep(time.Millisecond * 200)
	ztesting.Compare(t, "incorrect was ordered too late for delayed add to fix", sliceAsString(slice), "1,2,3,4,5,6,7,8,9,11,10,12")
}

func TestAll(t *testing.T) {
	testAdd(t)
	testSave(t)
	testLoad(t)
	testDelayed(t)
}
