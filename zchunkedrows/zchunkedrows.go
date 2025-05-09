//go:build server

package zchunkedrows

// ChunkedRows is a static list of byte-rows that is chunked into memory-mapped chunks.
// It can have an int64 for ordering if OrdererOffset option is set.
// If HasIncreasingIDFirstInRow option is set, adding rows automatically sets a int64 increasing id first in row.
// If AuxIndexOffset is set, at each row at that offset is a uint32 index into corresponding memory mapped chunk of auxillary data.
// If MatchIndexOffset is set, a chunk of lower-case match strings is also stored
// Then BinarySearch allows fast finding, and BinarySearchForChunk finds the chunk a value is in.
// AuxData and key can passed when adding a value. This is stored and saved atomically when the chunk with that value if first saved.

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zstr"
)

type ChunkPos string

const (
	PosNone         ChunkPos = ""
	PosEmpty        ChunkPos = "empty"
	PosWithin       ChunkPos = "within"
	PosAboveInChunk ChunkPos = "above"
	PosBelow        ChunkPos = "below"
	PosAboveOutside ChunkPos = "outside"
)

type LSOpts struct {
	RowsPerChunk              int
	RowByteSize               int
	DirPath                   string
	HasIncreasingIDFirstInRow bool // if true, an ID is increased and set first in row
	AuxIndexOffset            int  // if not 0, we have aux, and it is where aux chunk index is stored in row as a uint32
	MatchIndexOffset          int  // if not 0, we have match string chunks, and it is where index into this chunk is stored in row as a uint32
	OrdererOffset             int  // if not 0, where an uint32 to order rows is in a row
}

type ChunkedRows struct {
	// maps               map[chunkType]map[int]*os.File
	opts               LSOpts
	bottomChunkIndex   int
	topChunkIndex      int
	topChunkRowCount   int
	currentID          int64
	lock               deadlock.Mutex
	auxMatchRowEndChar byte // this should always be '\n', but can be changed for unit tests
	lastOrdererValue   int64
}

type chunkType int

const (
	isAux   chunkType = 1
	isRows  chunkType = 2
	isMatch chunkType = 4
)

var AboveError = errors.New("above")

var DefaultLSOpts = LSOpts{
	RowsPerChunk:              1024, // a million for events?
	HasIncreasingIDFirstInRow: true,
}

func New(opts LSOpts) *ChunkedRows {
	cr := &ChunkedRows{}
	cr.opts = opts
	zlog.Assert(cr.opts.DirPath != "")
	zfile.MakeDirAllIfNotExists(cr.opts.DirPath)
	cr.auxMatchRowEndChar = '\n'

	// cr.maps = map[chunkType]map[int]*os.File{}
	// cr.maps[isRows] = map[int]*os.File{}
	// if cr.opts.AuxIndexOffset != 0 {
	// 	cr.maps[isAux] = map[int]*os.File{}
	// }
	// if cr.opts.MatchIndexOffset != 0 {
	// 	cr.maps[isMatch] = map[int]*os.File{}
	// }
	err := cr.load()
	if zlog.OnError(err, cr.opts.DirPath) {
		return nil
	}
	return cr
}

func (cr *ChunkedRows) GetStorageSize() (rows, aux, match int64) {
	if cr.isEmpty() {
		return 0, 0, 0
	}
	for i := cr.bottomChunkIndex; i <= cr.topChunkIndex; i++ {
		rows += zfile.Size(cr.chunkFilepath(i, isRows))
		if cr.opts.MatchIndexOffset != 0 {
			match += zfile.Size(cr.chunkFilepath(i, isMatch))
		}
		if cr.opts.AuxIndexOffset != 0 {
			aux += zfile.Size(cr.chunkFilepath(i, isAux))
		}
	}
	return rows, aux, match
}

func (cr *ChunkedRows) totalRowCount() int {
	if cr.isEmpty() {
		return 0
	}
	l := (cr.topChunkIndex - cr.bottomChunkIndex) * cr.opts.RowsPerChunk
	l += cr.topChunkRowCount // the last chunk only has topChunkRowCount rows
	return l
}

func (cr *ChunkedRows) TotalRowCount() int {
	// cr.rwLock.RLock()
	n := cr.totalRowCount()
	// cr.rwLock.RUnlock()
	return n
}

func (cr *ChunkedRows) getOrderer(row []byte, isID bool) int64 {
	oi := cr.opts.OrdererOffset
	if isID {
		oi = 0
	}
	return int64(binary.LittleEndian.Uint64(row[oi:]))
}

func (c chunkType) String() string {
	switch c {
	case isAux:
		return "aux"
	case isRows:
		return "rows"
	case isMatch:
		return "match"
	}
	return ""
}

func (cr *ChunkedRows) chunkFilepath(i int, cType chunkType) string {
	name := fmt.Sprintf("%d.%s", i, cType)
	return zfile.JoinPathParts(cr.opts.DirPath, name)
}

// func (cr *ChunkedRows) closeMaps(chunkIndex int, remove bool) {
// 	for _, cType := range []chunkType{isAux, isRows, isMatch} {
// 		cmap := cr.maps[cType]
// 		if cmap == nil {
// 			continue
// 		}
// 		mm := cmap[chunkIndex]
// 		if mm != nil {
// 			mm.Close()
// 			delete(cmap, chunkIndex)
// 			// zlog.Warn("closeMap", chunkIndex, cType, cmap[chunkIndex])
// 		}
// 		if remove {
// 			// zlog.Warn("zChunkedRows.RemoveChunk:", chunkIndex)
// 			fpath := cr.chunkFilepath(chunkIndex, cType)
// 			os.Remove(fpath)
// 		}
// 	}
// }

func (cr *ChunkedRows) appendToChunkFile(chunkIndex int, cType chunkType, data []byte) (preFileLen int64, err error) {
	// fs, err := cr.getOrAddOutFile(chunkIndex, cType)
	// if zlog.OnError(err, chunkIndex, cType, fs) {
	// 	return 0, err
	// }

	file, err := cr.getChunkFile(chunkIndex, cType)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	preFileLen, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, zlog.Error("error seeking to end:", err, chunkIndex, cType)
	}
	n, err := file.Write(data)
	if err != nil {
		zlog.Error("write:", n, len(data), chunkIndex, preFileLen, isAux, err)
		return //0, zlog.Error("write:", chunkIndex, isAux, err)
	}
	if err == nil && n != len(data) {
		return 0, zlog.Error("wrote wrong size:", n, chunkIndex, isAux)
	}
	// path := cr.chunkFilepath(chunkIndex, cType)
	offset, err := file.Seek(0, os.SEEK_END)
	if cType == isRows {
		if err == nil && offset != int64(cr.topChunkRowCount*cr.opts.RowByteSize) {
			zlog.Error("file size and calculated row size not the same!", n, len(data), chunkIndex, offset, cr.topChunkRowCount*cr.opts.RowByteSize)
		}
	}
	if offset != preFileLen+int64(len(data)) {
		zlog.Error("Didn't add correct amount of bytes to chunk", offset-preFileLen, "!=", len(data), chunkIndex, preFileLen, offset)
	}
	// zlog.Info("appendRowToChunkMap", cType, cr.fileLengths[cType], "tcrc:", cr.topChunkRowCount, cr.topChunkRowCount*cr.opts.RowByteSize, "ci:", cr.topChunkIndex, len(data), err)
	return preFileLen, nil
}

func (cr *ChunkedRows) getChunkFile(chunkIndex int, cType chunkType) (*os.File, error) {
	// cmap := cr.maps[cType]
	// mm = cmap[chunkIndex]
	// if mm != nil {
	// 	return mm, nil
	// }
	fpath := cr.chunkFilepath(chunkIndex, cType)
	if zfile.NotExists(fpath) {
		f, err := os.Create(fpath)
		if err != nil {
			return nil, zlog.Error(err, fpath)
		}
		f.Close()
	}
	flags := os.O_RDONLY
	if chunkIndex == cr.topChunkIndex {
		flags = os.O_RDWR | os.O_CREATE
	}
	file, err := os.OpenFile(fpath, flags, 0644)
	// zlog.Warn("MMAP Open:", fpath, zdebug.CallingStackString())
	if err != nil {
		return nil, err
	}
	// zlog.Warn("getMemMap", cType, chunkIndex, cmap != nil)
	// cmap[chunkIndex] = mm
	return file, nil
}

// func (cr *ChunkedRows) CloseAllOutFiles() {
// 	cr.lock.Lock()
// 	for _, cType := range []chunkType{isAux, isRows, isMatch} {
// 		cr.closeOutFile(cType)
// 	}
// 	cr.lock.Unlock()
// }

func (cr *ChunkedRows) Close() {
	// cr.CloseAllOutFiles()
	// cr.rwLock.Lock()
	// if cr.delayAddTimer != nil {
	// 	cr.delayAddTimer.Stop()
	// }
	//cr.rwLock.Unlock()
}

// diffDir: 0 means it's in index chunk, 1 means chunk has bigger first value so goto before, 1 means last in chunk is smaller, go to next
func (cr *ChunkedRows) isOrdererInChunk(chunkIndex int, o int64, isIDOrderer bool) (diffDir int, err error) {
	if chunkIndex == cr.topChunkIndex {
		cr.lock.Lock()
		defer cr.lock.Unlock()
	}
	row := make([]byte, cr.opts.RowByteSize)
	file, err := cr.getChunkFile(chunkIndex, isRows)
	if zlog.OnError(err) {
		return zbool.Unknown, err
	}
	defer file.Close()
	err = cr.readRow(0, row, file)
	if zlog.OnError(err, zdebug.CallingStackString()) {
		return zbool.Unknown, err
	}
	ofirst := cr.getOrderer(row, isIDOrderer)
	// zlog.Warn("isInChunk", chunkIndex, o, ofirst)
	if ofirst == o { // we found exact match in first row in chunk
		return 0, nil
	}
	if ofirst > o { // first in chunk is bigger, return diffDir 1, we need to go to prev chunk
		return 1, nil
	}
	topRowIndex, _ := cr.getChunkRowCount(chunkIndex)
	topRowIndex--
	// zlog.Warn("isInChunk2", index, topRowIndex)
	err = cr.readRow(topRowIndex, row, file)
	if zlog.OnError(err) {
		return zbool.Unknown, err
	}
	olast := cr.getOrderer(row, isIDOrderer)
	if olast == o { // we found exact match in first row in chunk
		return 0, nil
	}
	if olast < o { // the last row is also smaller, so we need to go to next chunk
		return -1, nil
	}
	return 0, nil
}

func (cr *ChunkedRows) BinarySearchForChunk(find int64, isIDOrderer bool) (i int, pos ChunkPos, err error) {
	// cr.rwLock.RLock()
	// defer cr.rwLock.RUnlock()
	return cr.binarySearchForChunk(find, cr.bottomChunkIndex, cr.topChunkIndex, isIDOrderer)
}

func (cr *ChunkedRows) binarySearchForChunk(find int64, bottomChunkIndex, topChunkIndex int, isIDOrderer bool) (i int, pos ChunkPos, err error) {
	// zlog.Warn("binarySearchForChunk", find, bottomChunkIndex, topChunkIndex)
	if cr.isEmpty() {
		return 0, PosEmpty, nil
	}
	mid := (bottomChunkIndex + topChunkIndex) / 2
	diffDir, err := cr.isOrdererInChunk(mid, find, isIDOrderer)
	if err != nil {
		return 0, PosNone, err
	}
	// zlog.Warn("binarySearchForChunk2", find, bottomChunkIndex, topChunkIndex, mid, diffDir)
	if diffDir == 0 {
		return mid, PosWithin, nil
	}
	if diffDir > 0 {
		if bottomChunkIndex == topChunkIndex {
			return mid, PosBelow, nil
		}
		return cr.binarySearchForChunk(find, bottomChunkIndex, max(bottomChunkIndex, mid-1), isIDOrderer)
	}
	if bottomChunkIndex == topChunkIndex {
		// zlog.Warn("binarySearchForChunk2", find, bottomChunkIndex, topChunkIndex, "top:", cr.topChunkIndex, cr.topChunkRowCount, cr.opts.RowsPerChunk, topChunkIndex < cr.topChunkIndex)
		if topChunkIndex < cr.topChunkIndex {
			return mid + 1, PosBelow, nil
		}
		if cr.topChunkRowCount < cr.opts.RowsPerChunk {
			return mid, PosAboveInChunk, nil
		}
		return mid, PosAboveOutside, nil
	}
	// zlog.Warn("binarySearchForChunk4")
	return cr.binarySearchForChunk(find, min(topChunkIndex, mid+1), topChunkIndex, isIDOrderer)
}

func (cr *ChunkedRows) BinarySearch(find int64, isIDOrderer bool) (row []byte, chunkIndex, rowIndex int, exact bool, err error) {
	if cr.isEmpty() {
		return nil, 0, 0, false, nil
	}

	// zlog.Warn("BinarySearch", time.UnixMicro(find), isIDOrderer)
	var pos ChunkPos
	chunkIndex, pos, err = cr.binarySearchForChunk(find, cr.bottomChunkIndex, cr.topChunkIndex, isIDOrderer)
	// zlog.Warn("BinarySearch Got chunk", find, chunkIndex, pos, err)
	if err != nil {
		return nil, 0, 0, false, err
	}
	if chunkIndex == cr.topChunkIndex {
		cr.lock.Lock()
		defer cr.lock.Unlock()
	}
	file, err := cr.getChunkFile(chunkIndex, isRows)
	if err != nil {
		return nil, 0, 0, false, err
	}
	defer file.Close()
	if pos == PosAboveInChunk || pos == PosAboveOutside {
		row = make([]byte, cr.opts.RowByteSize)
		err = cr.readRow(cr.topChunkRowCount-1, row, file)
		zlog.OnError(err, find, isIDOrderer, cr.topChunkRowCount-1)
		return row, chunkIndex, cr.topChunkRowCount - 1, false, err
	}
	rowCount, _ := cr.getChunkRowCount(chunkIndex)
	// zlog.Warn("BinarySearch", cr.topChunkIndex, find, chunkIndex, "range", 0, rowCount-1)
	row, rowIndex, exact, err = cr.binarySearchForRow(find, file, 0, rowCount-1, rowCount-1, isIDOrderer)
	if err != nil {
		return nil, 0, 0, false, err
	}
	return row, chunkIndex, rowIndex, exact, nil
}

func (cr *ChunkedRows) binarySearchForRow(find int64, mm *os.File, bottomRowIndex, topRowIndex, maxRowIndex int, isIDOrderer bool) (row []byte, i int, exact bool, err error) {
	mid := (bottomRowIndex + topRowIndex) / 2
	// zlog.Warn("binarySearchForRow1", bottomRowIndex, mid, topRowIndex, "f:", find)
	row = make([]byte, cr.opts.RowByteSize)
	err = cr.readRow(mid, row, mm)
	if zlog.OnError(err) {
		return nil, 0, false, err
	}
	o := cr.getOrderer(row, isIDOrderer)
	// zlog.Warn("binarySearchForRow", bottomRowIndex, mid, topRowIndex, "f/o:", o, find, o > find, bottomRowIndex == topRowIndex)
	if o == find {
		return row, mid, true, nil
	}
	if o > find {
		if bottomRowIndex == topRowIndex {
			return row, mid, false, nil
		}
		return cr.binarySearchForRow(find, mm, bottomRowIndex, max(bottomRowIndex, mid-1), maxRowIndex, isIDOrderer)
	}
	if bottomRowIndex == topRowIndex {
		// zlog.Warn("AtTopBinary:", mid, maxRowIndex, find)
		// if mid == maxRowIndex {
		// 	return row, maxRowIndex + 1, false, nil
		// }
		return row, mid + 1, false, nil
	}
	return cr.binarySearchForRow(find, mm, min(topRowIndex, mid+1), topRowIndex, maxRowIndex, isIDOrderer)
}

func (cr *ChunkedRows) RowPosForIndex(i int) int {
	return i * cr.opts.RowByteSize
}

func (cr *ChunkedRows) TCRC() int {
	return cr.topChunkRowCount
}

func (cr *ChunkedRows) incrementRowOrChunk() {
	cr.currentID++
	if cr.topChunkRowCount < cr.opts.RowsPerChunk {
		cr.topChunkRowCount++
		return
	}
	// cr.CloseAllOutFiles()
	cr.topChunkRowCount = 1
	cr.topChunkIndex++
}

// Add keeps adding rows's to the top chunk, with optional aux data in aux chunks.
func (cr *ChunkedRows) Add(rowBytes []byte, auxData any) (int64, error) {
	var err error
	var match string
	var auxBytes []byte
	var auxPos int64 = -1
	var matchPos int64 = -1

	// prof := zlog.NewProfile(0.0001, "ChunkedRows Add:", len(rowBytes))
	// defer prof.End("")
	if cr.opts.OrdererOffset != 0 {
		o := int64(binary.LittleEndian.Uint64(rowBytes[cr.opts.OrdererOffset:]))
		if o < cr.lastOrdererValue && !zdebug.IsInTests {
			zlog.Error("zchunkRows.Add(): Added with orderer less than previous:", time.UnixMicro(cr.lastOrdererValue), (cr.lastOrdererValue-o)/1000, "ms", zlog.Full(auxData))
		}
		cr.lastOrdererValue = o
	}
	zlog.Assert((auxData != nil) == (cr.opts.AuxIndexOffset != 0), auxData != nil, cr.opts.AuxIndexOffset != 0)
	zlog.Assert((auxData != nil) == (cr.opts.MatchIndexOffset != 0), auxData != nil, cr.opts.MatchIndexOffset != 0)

	if auxData != nil {
		if cr.opts.MatchIndexOffset != 0 {
			mc, _ := auxData.(zstr.GetLowerCaseMatchContenter)
			if mc != nil {
				match = mc.GetLowerCaseMatchContent()
			} else {
				zlog.Assert(cr.opts.MatchIndexOffset == 0)
			}
		}
		idset, _ := (auxData).(zint.ID64Setter)
		if idset != nil {
			idset.SetID64(cr.currentID + 1)
		}
		djson, err := json.Marshal(auxData)
		if err != nil {
			return 0, zlog.Error(err, cr.topChunkIndex)
		}
		// zlog.Warn("IPO:", idset != nil, string(djson))
		auxBytes = append(djson, cr.auxMatchRowEndChar)
	}

	cr.lock.Lock()
	defer cr.lock.Unlock()

	cr.incrementRowOrChunk()
	if auxData != nil {
		if cr.opts.MatchIndexOffset != 0 {
			matchPos, err = cr.appendToChunkFile(cr.topChunkIndex, isMatch, []byte(match+string(cr.auxMatchRowEndChar)))
			if err != nil {
				return 0, zlog.Error(err, cr.topChunkIndex)
			}
		}
		auxPos, err = cr.appendToChunkFile(cr.topChunkIndex, isAux, auxBytes)
		if err != nil {
			cr.truncateChunk(isMatch, cr.topChunkIndex, matchPos)
			return 0, zlog.Error(err, cr.topChunkIndex, auxBytes != nil)
		}
	}

	if auxPos != -1 {
		binary.LittleEndian.PutUint32(rowBytes[cr.opts.AuxIndexOffset:], uint32(auxPos))
	}
	if matchPos != -1 {
		// zlog.Warn("Add: put index for match:", cr.opts.MatchIndexOffset, matchPos)
		binary.LittleEndian.PutUint32(rowBytes[cr.opts.MatchIndexOffset:], uint32(matchPos))
	}
	var id int64
	if cr.opts.HasIncreasingIDFirstInRow {
		id = cr.currentID
		binary.LittleEndian.PutUint64(rowBytes[0:], uint64(id))
	}
	_, err = cr.appendToChunkFile(cr.topChunkIndex, isRows, rowBytes)
	if err != nil {
		zlog.Info("appendToChunkFile Err", err, cr.topChunkIndex, isRows, len(rowBytes))
		cr.topChunkRowCount--
		cr.currentID--
		cr.truncateChunk(isAux, cr.topChunkIndex, auxPos)
		cr.truncateChunk(isMatch, cr.topChunkIndex, matchPos)
		return 0, err
	}
	return id, nil
}

func (cr *ChunkedRows) truncateChunk(cType chunkType, chunkIndex int, toPos int64) error {
	zlog.Error("truncateChunk", cType, chunkIndex, toPos)
	if toPos == -1 {
		return nil
	}
	path := cr.chunkFilepath(chunkIndex, cType)
	f, err := os.Open(path)
	if zlog.OnError(err) {
		return err
	}
	f.Truncate(toPos)
	f.Seek(toPos, io.SeekStart)
	f.Close()
	return nil
}

func (cr *ChunkedRows) deleteChunk(i int) error {
	cTypes := []chunkType{isRows}
	if cr.opts.MatchIndexOffset != 0 {
		cTypes = append(cTypes, isMatch)
	}
	if cr.opts.AuxIndexOffset != 0 {
		cTypes = append(cTypes, isAux)
	}
	for _, cType := range cTypes {
		fpath := cr.chunkFilepath(i, cType)
		err := os.Remove(fpath)
		zlog.OnError(err, fpath)
	}
	if i == cr.bottomChunkIndex {
		cr.bottomChunkIndex++
	}
	if i == cr.topChunkIndex {
		cr.topChunkRowCount = 0
	}
	return nil
}

func (cr *ChunkedRows) isEmpty() bool {
	return cr.bottomChunkIndex == cr.topChunkIndex && cr.topChunkRowCount == 0
}

func (cr *ChunkedRows) load() error {
	zfile.MakeDirAllIfNotExists(cr.opts.DirPath)
	cr.lock.Lock()
	defer cr.lock.Unlock()
	cTypes := []chunkType{isAux, isRows, isMatch}

	var ranges = map[chunkType]zmath.Range[int]{}
	zfile.Walk(cr.opts.DirPath, "", zfile.WalkOptionGiveNameOnly, func(fname string, info os.FileInfo) error {
		var sn, stub string
		if zstr.SplitN(fname, ".", &sn, &stub) {
			n, err := strconv.Atoi(sn)
			if zlog.OnError(err, sn) {
				return nil
			}
			for _, cType := range cTypes {
				if stub == cType.String() {
					ranges[cType] = ranges[cType].Added(n)
				}
			}
		}
		return nil
	})
	if !ranges[isRows].Valid {
		zlog.Info("Deleting zchunkedrows dir with invalid chunk range (empty)", cr.opts.DirPath)
		zfile.RemoveContents(cr.opts.DirPath)
		cr.currentID = 0
		return nil
	}
	mins := zmath.GetRangeMins(zmap.AllValues(ranges))
	for cr.bottomChunkIndex = slices.Min(mins); cr.bottomChunkIndex < slices.Max(mins); cr.bottomChunkIndex++ { // if more aux chunks than chunk chunks or visa versa at bottom
		zlog.Warn("zchunkedrows deleting bottom chunk without matching aux/match range", cr.bottomChunkIndex, cr.opts.DirPath)
		cr.deleteChunk(cr.bottomChunkIndex)
	}
	maxes := zmath.GetRangeMaxes(zmap.AllValues(ranges))
	for cr.topChunkIndex = slices.Max(maxes); cr.topChunkIndex > slices.Min(maxes); cr.topChunkIndex-- { // likewise for top
		zlog.Warn("zchunkedrows deleting top chunk without matching aux/match range", cr.bottomChunkIndex, cr.opts.DirPath)
		cr.deleteChunk(cr.topChunkIndex)
	}
	// zlog.Warn("Loaded:", maxes, cr.bottomChunkIndex, cr.topChunkIndex)
	file, err := cr.getChunkFile(cr.topChunkIndex, isRows)
	if err != nil {
		return zlog.Error(err, cr.topChunkIndex)
	}
	err = cr.handleLoadedTopRow(file)
	file.Close()
	//TODO: Check if top (or all) aux and row chunks have same top value(s)

	return nil
}

func (cr *ChunkedRows) handleLoadedTopRow(file *os.File) error {
	var hasBadChunkAbove bool
	cr.topChunkRowCount, hasBadChunkAbove = cr.getChunkRowCount(cr.topChunkIndex)

	topRowIndexOnLoad = cr.topChunkRowCount
	lastRow := make([]byte, cr.opts.RowByteSize)
	err := cr.readRow(cr.topChunkRowCount-1, lastRow, file)
	if zlog.OnError(err) {
		return err
	}
	cTypes := map[int]chunkType{cr.opts.MatchIndexOffset: isMatch, cr.opts.AuxIndexOffset: isAux}
	for offset, ctype := range cTypes {
		if offset == 0 {
			continue
		}
		_, _, err := cr.getLineFromChunk(cr.topChunkIndex, offset, ctype, lastRow, nil)
		if err != nil {
			hasBadChunkAbove = true
			cr.topChunkRowCount--
			break
		}
	}
	if hasBadChunkAbove {
		err = cr.readRow(cr.topChunkRowCount-1, lastRow, file)
		if zlog.OnError(err) {
			return err
		}
		for offset, ctype := range cTypes {
			if offset == 0 {
				continue
			}
			_, endPos, err := cr.getLineFromChunk(cr.topChunkIndex, offset, ctype, lastRow, nil)
			if err != nil {
				return err
			}
			cr.truncateChunk(ctype, cr.topChunkIndex, endPos)
		}
		// cr.closeMaps(cr.topChunkIndex, false)
	}
	cr.currentID = int64(binary.LittleEndian.Uint64(lastRow[0:]))
	return nil
}

func (cr *ChunkedRows) getChunkRowCount(chunkIndex int) (top int, hasBadChunkAbove bool) {
	fpath := cr.chunkFilepath(chunkIndex, isRows)
	size := zfile.Size(fpath)
	top = int(size) / cr.opts.RowByteSize
	hasBadChunkAbove = (size%int64(cr.opts.RowByteSize) != 0)
	return top, hasBadChunkAbove
}

var topRowIndexOnLoad int

func (cr *ChunkedRows) GetAuxDataUnlocked(chunkIndex int, row []byte, dataPtr any, cachedFile **os.File) error {
	bjson, _, err := cr.getLineFromChunk(chunkIndex, cr.opts.AuxIndexOffset, isAux, row, cachedFile)
	if err != nil {
		return zlog.NewError(err, chunkIndex)
	}
	err = json.Unmarshal(bjson, dataPtr)
	if err != nil {
		return zlog.NewError("unmarshal-json:", err, chunkIndex, "json:", zstr.Head(string(bjson), 2000), zdebug.CallingStackString())
	}
	return nil
}

func (cr *ChunkedRows) GetAuxData(chunkIndex int, row []byte, dataPtr any, cachedFile **os.File) error {
	if chunkIndex == cr.topChunkIndex {
		cr.lock.Lock()
		cr.lock.Unlock()
	}
	err := cr.GetAuxDataUnlocked(chunkIndex, row, dataPtr, cachedFile)
	return err
}

// getMatchStr gets the string with text to search-match the row with. It gets it from the *.match file based on
// position at MatchIndexOffset in row. It can reuse cachedFile if points to something,
// but this has to be made to point to nil for each new chunkIndex by caller.
func (cr *ChunkedRows) getMatchStr(chunkIndex int, row []byte, cachedFile **os.File) (string, error) {
	matchBytes, _, err := cr.getLineFromChunk(chunkIndex, cr.opts.MatchIndexOffset, isMatch, row, cachedFile)
	if err != nil {
		return "", zlog.NewError(err, chunkIndex)
	}
	return string(matchBytes), nil
}

func (cr *ChunkedRows) onErrorRemoveChunkMapFileIfFirstGet(chunkIndex int, cType chunkType) {
	if chunkIndex != 2 { // for now to limit prints
		return
	}
	zlog.Warn("onErrorRemoveChunkMapFileIfFirstGet1", chunkIndex, cType, cr.topChunkIndex, cr.topChunkRowCount, zdebug.CallingStackString())
	if chunkIndex == cr.topChunkIndex && cr.topChunkRowCount == 0 {
		zlog.Warn("onErrorRemoveChunkMapFileIfFirstGet:", chunkIndex, cType)
		// cr.closeMaps(chunkIndex, true)
	}
}

var firstError = true

func (cr *ChunkedRows) getLineFromChunk(chunkIndex, offset int, cType chunkType, row []byte, cachedFile **os.File) (lineBytes []byte, endPos int64, err error) {
	var file *os.File
	if cachedFile != nil {
		file = *cachedFile
	}
	if file == nil {
		file, err = cr.getChunkFile(chunkIndex, cType)
		if file != nil && cachedFile != nil {
			*cachedFile = file
		}
	}
	if err != nil {
		return nil, 0, zlog.NewError(err, chunkIndex, offset, cType)
	}
	i := binary.LittleEndian.Uint32(row[offset : offset+4])

	_, err = file.Seek(int64(i), io.SeekStart)
	// zlog.Warn("getLineFromChunk", i, err, chunkIndex, offset, cType)
	if err != nil {
		cr.onErrorRemoveChunkMapFileIfFirstGet(chunkIndex, cType)
		return nil, 0, zlog.NewError(err, i, chunkIndex, offset, cType)
	}
	reader := bufio.NewReader(file)
	lineBytes, err = reader.ReadBytes(cr.auxMatchRowEndChar)
	if err != nil {
		// zlog.Error("chunk read fail:", len(lineBytes), "seek:", i, err)
		cr.onErrorRemoveChunkMapFileIfFirstGet(chunkIndex, cType)
		return nil, 0, err
	}
	lineBytes = lineBytes[:len(lineBytes)-1]
	// scanner := bufio.NewScanner(file)
	// if !scanner.Scan() {
	// 	return nil, 0, zlog.NewError("Error scanning chunk:", scanner.Err(), i, offset, cType, chunkIndex)
	// }
	// lineBytes = scanner.Bytes()
	endPos = int64(i) + int64(len(lineBytes)) + 1
	return lineBytes, endPos, nil
}

func (cr *ChunkedRows) readRow(index int, bytes []byte, file *os.File) error {
	// zlog.Warn("readRow:", index)
	n, err := file.ReadAt(bytes, int64(index*cr.opts.RowByteSize))
	if n != cr.opts.RowByteSize || err != nil {
		return zlog.Error("couldn't read row:", index, n, cr.opts.RowByteSize, err) // , zdebug.CallingStackString())
	}
	return nil
}

// Iterate  starts in chunk startChunkIndex at row indexInRow. If skipFunc<>nil and returns true for the row,
// and match <> "" and is contained in the row's match text, got is called.
// If got returns true, the next or previous row is used depending on forward.
func (cr *ChunkedRows) Iterate(startChunkIndex, indexInRow int, forward bool, match string, skipFunc func(row []byte) bool, got func(row []byte, chunkIndex, index int, err error) bool) (totalRows int, err error) {
	if cr.isEmpty() {
		return 0, nil
	}
	if match != "" {
		zlog.Assert(cr.opts.MatchIndexOffset != 0)
	}
	match = strings.ToLower(match)
	// zlog.Warn("Iter1:", cr.bottomChunkIndex, cr.topChunkIndex, cr.topChunkRowCount, "in:", startChunkIndex, index, forward)
	chunkIndex := startChunkIndex
	if indexInRow >= cr.opts.RowsPerChunk {
		return 0, zlog.NewError("index too big for chunk", indexInRow, cr.opts.RowsPerChunk)
	}
	if startChunkIndex == -1 {
		if forward {
			chunkIndex = cr.bottomChunkIndex
			if indexInRow < 0 || indexInRow >= cr.opts.RowsPerChunk {
				return 0, zlog.NewError("index outside range:", indexInRow)
			}
		} else {
			chunkIndex = cr.topChunkIndex
			if indexInRow == -1 {
				indexInRow = cr.topChunkRowCount - 1
			}
		}
	} else {
		if startChunkIndex > cr.topChunkIndex {
			return 0, zlog.NewError("startChunkIndex after topChunkIndex", startChunkIndex, cr.bottomChunkIndex)
		}
		if startChunkIndex < cr.bottomChunkIndex {
			return 0, zlog.NewError("startChunkIndex before bottomChunkIndex", startChunkIndex, cr.bottomChunkIndex)
		}
		if indexInRow < 0 {
			return 0, zlog.NewError("index < 0 for chunk not -1", indexInRow, cr.opts.RowsPerChunk)
		}
		if startChunkIndex == cr.topChunkIndex && indexInRow >= cr.topChunkRowCount {
			return 0, zlog.NewError("index after top row in top chunk", startChunkIndex, "==", cr.topChunkIndex, indexInRow, ">=", cr.topChunkRowCount)
		}
	}
	row := make([]byte, cr.opts.RowByteSize)
	oldChunkIndex := -1
	count := 0
	var file, matchFile *os.File
	for {
		// if count == 4444 {
		// 	zlog.Info("chunked.procfiles:", strings.Join(zprocess.GetOpenDiskFileNames(), "\n"))
		// }
		if count%500000 == 0 && count != 0 {
			zlog.Info("chunked.Iterate: count:", count, "ci:", chunkIndex, "i:", indexInRow, match)
		}
		var err error
		count++
		if oldChunkIndex != chunkIndex {
			if file != nil {
				file.Close()
			}
			matchFile = nil // we need to open a new file
			file, err = cr.getChunkFile(chunkIndex, isRows)
			if zlog.OnError(err, chunkIndex) {
				got(row, chunkIndex, indexInRow, err)
				return 0, err
			}
			oldChunkIndex = chunkIndex
		}
		err = cr.readRow(indexInRow, row, file)
		skip := false
		if err != nil {
			skip = true
			got(row, chunkIndex, indexInRow, err)
			zlog.Warn("iter.ReadRow: err", chunkIndex, indexInRow, err)
		} else {
			if skipFunc != nil {
				skip = skipFunc(row)
			}
			if !skip && match != "" {
				str, err := cr.getMatchStr(chunkIndex, row, &matchFile)
				// zlog.Warn("iter.getMatch", chunkIndex, index, str, err)
				if err != nil {
					skip = true
					got(row, chunkIndex, indexInRow, err)
					// zlog.Warn("iter.ReadRow: getMatch err", chunkIndex, index, err)
					continue
				}
				if !strings.Contains(str, match) {
					skip = true
				}
			}
		}
		if !skip && !got(row, chunkIndex, indexInRow, nil) {
			break
		}
		if forward {
			indexInRow++
			if chunkIndex == cr.topChunkIndex && indexInRow >= cr.topChunkRowCount {
				break
			}
			if indexInRow >= cr.opts.RowsPerChunk {
				indexInRow = 0
				chunkIndex++
				if chunkIndex > cr.topChunkIndex {
					break
				}
				file.Close()
				file = nil
			}
		} else {
			indexInRow--
			if indexInRow < 0 {
				indexInRow = cr.opts.RowsPerChunk - 1
				chunkIndex--
				if chunkIndex < cr.bottomChunkIndex {
					break
				}
				file.Close()
				file = nil
			}
		}
	}
	if file != nil {
		file.Close()
	}
	return count, nil
}

// PosForIndexes returns a combined fixed position of chunkIndex and rowIndex
func (cr *ChunkedRows) PosForIndexes(chunkIndex, rowIndex int) int {
	return chunkIndex*cr.opts.RowsPerChunk + rowIndex
}

// IndexesFromPos converts the position back to chunk and row indexes
func (cr *ChunkedRows) IndexesFromPos(pos int) (chunkIndex, rowIndex int) {
	if pos == -1 {
		return -1, -1
	}
	chunkIndex = pos / cr.opts.RowsPerChunk
	rowIndex = pos % cr.opts.RowsPerChunk
	return chunkIndex, rowIndex
}

func (cr *ChunkedRows) SetRowsPerChunkAtStart(rowsPerChunk int) {
	cr.opts.RowsPerChunk = rowsPerChunk
}

type FS struct {
	file *os.File
	size int64
}

func (cr *ChunkedRows) DeleteChunksOlderThan(old time.Time) error {
	isIDOrderer := false
	t := old.UnixMicro()
	zlog.Info("ChunkedRows.DeleteChunksOlderThan:", old)
	index, cpos, err := cr.binarySearchForChunk(t, cr.bottomChunkIndex, cr.topChunkIndex, isIDOrderer)
	zlog.Info("ChunkedRows.DeleteChunksOlderThan2:", index, cpos, err)
	if err != nil {
		return zlog.Error(err, t)
	}
	if (cpos != PosWithin && cpos != PosBelow) || index == cr.topChunkIndex {
		return nil
	}
	for i := cr.bottomChunkIndex; i < index; i++ {
		cr.deleteChunk(i)
	}
	// zlog.Warn("DeleteChunksOlderThan: left", cr.totalRowCount(), cr.bottomChunkIndex, cr.topChunkIndex)
	return nil
}
