//go:build !js

package zchunkedrows

import (
	"encoding/binary"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
	"github.com/torlangballe/zutil/ztime"
)

const rowSize = 24

type Event struct {
	ID   int64
	Time int64
	Text string
}

func (e Event) GetLowerCaseMatchContent() string {
	return strings.ToLower(e.Text)
}

func (e *Event) SetID64(id int64) {
	e.ID = id
}

func checkBinarySearchForChunk(t *testing.T, chunkedRows *ChunkedRows, val int64, wantChunkIndex int, wantPos ChunkPos) {
	index, cpos, err := chunkedRows.BinarySearchForChunk(val, false)
	// zlog.Warn("checkBinarySearchForChunk", val, index, cpos, wantPos, err)
	if err != nil {
		t.Error(err)
		return
	}
	ztesting.Equal(t, index, wantChunkIndex, "checkBinarySearchForChunk", val)
	ztesting.Equal(t, cpos, wantPos, "checkBinarySearchForChunk:", val)
}

func checkBinarySearch(t *testing.T, chunkedRows *ChunkedRows, val int64, wantChunkIndex, wantIndex int, exact bool) {
	_, chunkIndex, index, ex, err := chunkedRows.BinarySearch(val, false)
	if err != nil {
		if err != AboveError {
			t.Error(err)
			return
		}
	}
	ztesting.Equal(t, chunkIndex, wantChunkIndex, "checkBinarySearch chunk:", val)
	ztesting.Equal(t, index, wantIndex, "checkBinarySearch index:", val)
	ztesting.Equal(t, ex, exact, "checkBinarySearch", val, "accurate:")
}

func makeChunkedRows(path string) *ChunkedRows {
	opts := DefaultLSOpts
	opts.RowsPerChunk = 5
	opts.RowByteSize = rowSize
	opts.OrdererOffset = 8
	if path == "" {
		path = zfile.CreateTempFilePath("zchunkedrows-test")
	}
	opts.DirPath = path
	// zlog.Warn("folder:", opts.DirPath)
	return New(opts)
}

func makeEventBytes(e Event) []byte {
	row := make([]byte, rowSize)
	binary.LittleEndian.PutUint64(row[0:], uint64(e.ID))
	binary.LittleEndian.PutUint64(row[8:], uint64(e.Time))
	return row
}

func eventFromBytes(row []byte) Event {
	var e Event
	e.ID = int64(binary.LittleEndian.Uint64(row[0:]))
	e.Time = int64(binary.LittleEndian.Uint64(row[8:]))
	return e
}

func addNumber(cr *ChunkedRows, n int) {
	e := Event{Time: int64(n)}
	row := makeEventBytes(e)
	cr.Add(row, nil)
}
func addStandardNumbers(chunkedRows *ChunkedRows) {
	for i := 1; i < 35; i += 3 {
		addNumber(chunkedRows, i)
	}
}
func testAdd(t *testing.T) {
	zlog.Warn("testAdd")
	chunkedRows := makeChunkedRows("")
	addStandardNumbers(chunkedRows)
	// zlog.Warn("Items:", chunkedRowsAsString(chunkedRows))
	// 1 4 7 10 13 - 16 19 22 25 28 - 31 34
	// fmt.Println(chunkedRows.opts.DirPath)

	ztesting.Equal(t, chunkedRows.TotalRowCount(), 12, "Len")
	// fmt.Println("All:", chunkedRowsAsString(chunkedRows))
}

func testBinarySearch(t *testing.T) {
	zlog.Warn("testBinarySearch")
	chunkedRows := makeChunkedRows("")
	addStandardNumbers(chunkedRows)

	// zlog.Warn("All:", chunkedRows.TotalRowCount())
	checkBinarySearchForChunk(t, chunkedRows, 0, 0, PosBelow)
	checkBinarySearchForChunk(t, chunkedRows, 1, 0, PosWithin)
	checkBinarySearchForChunk(t, chunkedRows, 5, 0, PosWithin)
	checkBinarySearchForChunk(t, chunkedRows, 14, 1, PosBelow)
	checkBinarySearchForChunk(t, chunkedRows, 28, 1, PosWithin)
	checkBinarySearchForChunk(t, chunkedRows, 34, 2, PosWithin)
	checkBinarySearchForChunk(t, chunkedRows, 35, 2, PosAboveInChunk)

	addNumber(chunkedRows, 38)
	addNumber(chunkedRows, 39)
	addNumber(chunkedRows, 40)

	checkBinarySearchForChunk(t, chunkedRows, 40, 2, PosWithin)
	checkBinarySearchForChunk(t, chunkedRows, 41, 2, PosAboveOutside)

	checkBinarySearch(t, chunkedRows, 0, 0, 0, false)
	checkBinarySearch(t, chunkedRows, 1, 0, 0, true)
	checkBinarySearch(t, chunkedRows, 3, 0, 1, false)
	checkBinarySearch(t, chunkedRows, 4, 0, 1, true)
	checkBinarySearch(t, chunkedRows, 7, 0, 2, true)
	checkBinarySearch(t, chunkedRows, 13, 0, 4, true)
	checkBinarySearch(t, chunkedRows, 14, 1, 0, false)
	checkBinarySearch(t, chunkedRows, 16, 1, 0, true)

	checkBinarySearch(t, chunkedRows, 19, 1, 1, true)
	checkBinarySearch(t, chunkedRows, 28, 1, 4, true)
	checkBinarySearch(t, chunkedRows, 29, 2, 0, false)
	checkBinarySearch(t, chunkedRows, 31, 2, 0, true)
	checkBinarySearch(t, chunkedRows, 33, 2, 1, false)
	checkBinarySearch(t, chunkedRows, 34, 2, 1, true)
	checkBinarySearch(t, chunkedRows, 35, 2, 2, false)
	checkBinarySearch(t, chunkedRows, 37, 2, 2, false)
}

func testLoad(t *testing.T) {
	zlog.Warn("testLoad")
	chunkedRows := makeChunkedRows("")
	addStandardNumbers(chunkedRows)

	chunkedRows2 := makeChunkedRows(chunkedRows.opts.DirPath)

	ztesting.Equal(t, chunkedRows.TotalRowCount(), chunkedRows2.TotalRowCount(), "loaded len is same")
}

func joinOrderForIterator(chunkedRows *ChunkedRows, startChunkIndex, index int, forward bool) string {
	var orderers []string
	_, err := chunkedRows.Iterate(startChunkIndex, index, forward, "", func(row []byte, chunkIndex, index int) bool {
		o := chunkedRows.getOrderer(row, false)
		so := fmt.Sprint(o)
		if chunkedRows.opts.MatchIndexOffset != 0 {
			m, err := chunkedRows.getMatchStr(chunkIndex, row)
			so += m
			if err != nil {
				so += "<" + err.Error() + ">"
			}
		}
		orderers = append(orderers, so)
		return true
	})
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return strings.Join(orderers, ",")
}

func chunkedRowsAsString(chunkedRows *ChunkedRows) string {
	return joinOrderForIterator(chunkedRows, -1, 0, true)
}

func testIterate(t *testing.T) {
	zlog.Warn("testIterate")
	chunkedRows := makeChunkedRows("")
	addStandardNumbers(chunkedRows)

	str := joinOrderForIterator(chunkedRows, -1, -1, false)
	ztesting.Equal(t, "34,31,28,25,22,19,16,13,10,7,4,1", str, "backwards iterate all not same")

	str = joinOrderForIterator(chunkedRows, 0, -1, false)
	ztesting.Equal(t, "<index < 0 for chunk not -1 -1 5>", str, "should fail on index outside too small")

	str = joinOrderForIterator(chunkedRows, 0, 6, false)
	ztesting.Equal(t, "<index too big for chunk 6 5>", str, "should fail on index outside too big")

	str = joinOrderForIterator(chunkedRows, 1, 0, true)
	ztesting.Equal(t, str, "16,19,22,25,28,31,34", "1) should be end of rows")

	str = joinOrderForIterator(chunkedRows, 1, 1, true)
	ztesting.Equal(t, str, "19,22,25,28,31,34", "2) should be end of rows")

	str = joinOrderForIterator(chunkedRows, 2, 1, true)
	ztesting.Equal(t, str, "34", "3)should be end of rows")

	str = joinOrderForIterator(chunkedRows, 1, 3, false)
	ztesting.Equal(t, str, "25,22,19,16,13,10,7,4,1", "1) should be head of rows")
}

func testCorruption(t *testing.T) {
	zlog.Warn("testCorruption")
	const count = 5
	names := []string{"john", "sally", "bill", "fred", "jill", "peter", "tor", "paul"}
	opts := DefaultLSOpts
	opts.RowsPerChunk = 5
	opts.RowByteSize = rowSize
	opts.OrdererOffset = 8
	opts.MatchIndexOffset = 16
	opts.AuxIndexOffset = 20
	opts.DirPath = zfile.CreateTempFilePath("zchunkedrows-test")
	chunkedRows := New(opts)

	for i, n := range names {
		if i == len(names)-1 {
			chunkedRows.auxMatchRowEndChar = '\t'
		}
		e := &Event{Time: int64(i + 1), Text: n}
		row := makeEventBytes(*e)
		chunkedRows.Add(row, e)
	}
	chunkedRows.auxMatchRowEndChar = '\n'
	str := chunkedRowsAsString(chunkedRows)
	ztesting.Equal(t, str, "1john,2sally,3bill,4fred,5jill,6peter,7tor,8<1: EOF>", "all five with match")
	chunkedRows2 := New(opts)
	return
	str = chunkedRowsAsString(chunkedRows2)
	ztesting.Equal(t, str, "1john,2sally,3bill,4fred,5jill,6peter,7tor", "last row with error truncated")
}

func testDeleteOldChunk(t *testing.T) {
	zlog.Warn("testDeleteOldChunk")
	chunkedRows := makeChunkedRows("")
	var end time.Time
	for i := 0; i < 100; i++ {
		t := time.Now().Add(time.Millisecond * time.Duration(i))
		end = t
		u := t.UnixMicro()
		e := Event{Time: u}
		row := makeEventBytes(e)
		chunkedRows.Add(row, nil)
	}
	sum := chunkedRows.TotalRowCount()
	ztesting.Equal(t, sum, 100, "100 before delete")
	chunkedRows.DeleteOldChunksThan(end.Add(-time.Millisecond * 20))
	sumAfter := chunkedRows.TotalRowCount()
	ztesting.Equal(t, sumAfter, 20, "20 after delete")
}

func testBadOrder(t *testing.T) {
	const count = 100
	zlog.Warn("testBadOrder")
	chunkedRows := makeChunkedRows("")
	now := time.Now()
	for i := 0; i < 100; i++ {
		t := now.Add(ztime.SecondsDur(float64(rand.Int31n(3600))))
		u := t.UnixMicro()
		e := Event{Time: u}
		row := makeEventBytes(e)
		chunkedRows.Add(row, nil)
	}
	for i := 0; i < 50; i++ {
		t := now.Add(-time.Minute).Add(ztime.SecondsDur(float64(rand.Int31n(3660))))
		u := t.UnixMicro()
		_, ci, ri, exact, err := chunkedRows.BinarySearch(u, false)
		zlog.Warn("Find", t, ":", ci, ri, exact, err)
	}
}

func TestAll(t *testing.T) {
	testAdd(t)
	testBinarySearch(t)
	testLoad(t)
	testIterate(t)
	testCorruption(t)
	testDeleteOldChunk(t)
	testBadOrder(t)
}
