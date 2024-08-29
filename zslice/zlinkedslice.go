//go:build server

package zslice

// LinkedSlice[T,O] is a static generic slice of T that is broken up into sub-slices with a fixed length.
// It has Len() and Index() methods to randomly access any item, and Add() to add value.
// In its simplest case the values are not ordered, and O is not used.
// For ordered data O must refer to something that can be extracted from T and used for search/sort.
// Then BinarySearch allows fast finding, and BinarySearchForChunk finds the chunk a value is in.
// For ordered data, if DelaySortAddSecs is not 0, Add'ed values are stored for time, and sorted by T before being added.
// If DB is set, the current chunk can be saved with saveLastChunk, which is automatically called when a chunk becomes full.
// If SaveSecs is set, saving can be done automatically periodically if anything has been added.
// AuxData and key can passed when adding a value. This is stored and saved atomically when the chunk with that value if first saved.

import (
	"bytes"
	"cmp"
	"encoding/gob"
	"fmt"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/lotusdblabs/lotusdb/v2"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zbytes"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type LSOpts struct {
	ChunkSize        int
	SaveSecs         float64
	DelaySortAddSecs float64
	DB               *lotusdb.DB
}

type LinkedSlice[T any, O cmp.Ordered] struct {
	chunks                [][]T
	opts                  LSOpts
	getOrdererFunc        func(T) O
	delayedAdds           []delayedAdd[T]
	saveTimer             *ztimer.Repeater
	delayAddTimer         *ztimer.Repeater
	currentChunkSaveIndex int
	lock                  sync.Mutex
	touched               bool
	auxToSaveWithChunk    map[string]any
	binaryCompareFunc     func(t T, o O) int

	Loaded     bool
	BottomLock sync.Mutex

	GetIDFunc func(T) int64
}

var DefaultLSOpts = LSOpts{
	ChunkSize: 1024,
	SaveSecs:  1,
}

func NewLinked[T any, O cmp.Ordered](opts LSOpts, getOrderer func(T) O) *LinkedSlice[T, O] {
	ls := &LinkedSlice[T, O]{}
	ls.opts = opts
	ls.getOrdererFunc = getOrderer
	if ls.opts.DB != nil {
		ls.saveTimer = ztimer.RepeatForever(1, func() {
			ls.lock.Lock()
			if ls.touched {
				ls.touched = false
				ls.saveLastChunk()
			}
			ls.lock.Unlock()
		})
	}
	ls.binaryCompareFunc = func(t T, o O) int {
		ot := getOrderer(t)
		if ot < o {
			return -1
		}
		if ot > o {
			return 1
		}
		return 0
	}
	if ls.opts.DelaySortAddSecs != 0 {
		ls.delayAddTimer = ztimer.RepeatForever(ls.opts.DelaySortAddSecs, func() {
			ls.lock.Lock()
			ls.setDelayedItems(false)
			ls.lock.Unlock()
		})
	}
	ls.auxToSaveWithChunk = map[string]any{}
	return ls
}

func (ls *LinkedSlice[T, O]) setDelayedItems(forceAll bool) {
	var old []delayedAdd[T]
	if forceAll {
		old = ls.delayedAdds
		ls.delayedAdds = ls.delayedAdds[:0]
	} else {
		delay := time.Duration(float64(time.Second) * ls.opts.DelaySortAddSecs)
		var found bool
		for i, d := range ls.delayedAdds {
			if time.Since(d.added) < delay {
				old = ls.delayedAdds[:i]
				ls.delayedAdds = ls.delayedAdds[i:]
				found = true
				break
			}
		}
		if !found {
			old = ls.delayedAdds
			ls.delayedAdds = ls.delayedAdds[:0]
		}
	}
	if len(old) == 0 {
		return
	}
	sortFunc := func(i, j int) bool {
		oi := ls.getOrdererFunc(old[i].item)
		oj := ls.getOrdererFunc(old[j].item)
		return oi < oj
	}
	sort.Slice(old, sortFunc)
	for _, o := range old {
		ls.add(o.item, o.auxData)
	}
}

func (ls *LinkedSlice[T, O]) Len() int {
	ls.lock.Lock()
	n := ls.len()
	ls.lock.Unlock()
	return n
}

func (ls *LinkedSlice[T, O]) len() int {
	count := len(ls.chunks)
	if count == 0 {
		return 0
	}
	len := (count-1)*ls.opts.ChunkSize + len(ls.chunks[count-1])
	return len
}

func (ls *LinkedSlice[T, O]) Index(i int) *T {
	ci := i / ls.opts.ChunkSize
	si := i % ls.opts.ChunkSize
	return &ls.chunks[ci][si]
}

func (ls *LinkedSlice[T, O]) Flush() {
	ls.lock.Lock()
	ls.setDelayedItems(true)
	if ls.opts.DB != nil {
		if ls.saveTimer != nil {
			ls.saveTimer.Stop()
		}
		ls.saveLastChunk()
	}
	ls.lock.Unlock()
}

func (ls *LinkedSlice[T, O]) Close() {
	ls.Flush()
	ls.lock.Lock()
	if ls.opts.DB != nil {
		if ls.saveTimer != nil {
			ls.saveTimer.Stop()
		}
		err := ls.opts.DB.Close()
		zlog.OnError(err)
	}
	if ls.delayAddTimer != nil {
		ls.saveTimer.Stop()
	}
	ls.chunks = [][]T{}
	ls.lock.Unlock()
}

func (ls *LinkedSlice[T, O]) BinarySearchForChunk(find O) (i int, before zbool.BoolInd) {
	ls.lock.Lock()
	i, before = ls.binarySearchForChunk(find)
	ls.lock.Unlock()
	return i, before
}

func (ls *LinkedSlice[T, O]) binarySearchForChunk(find O) (i int, before zbool.BoolInd) {
	lsLen := len(ls.chunks)
	if lsLen == 0 {
		return 0, zbool.Unknown
	}
	for i, c := range ls.chunks {
		if ls.getOrdererFunc(c[0]) > find {
			return i, zbool.True
		}
		if ls.getOrdererFunc(c[len(c)-1]) < find {
			continue
		}
		return i, zbool.Unknown
	}
	// fmt.Println("BinarySearchForChunk at end", f, lsLen-1)
	return lsLen - 1, zbool.False
}

func (ls *LinkedSlice[T, F]) BinarySearch(f F) (i int, exact bool) {
	ls.lock.Lock()
	defer ls.lock.Unlock()
	chunksLen := len(ls.chunks)
	if chunksLen == 0 {
		return 0, false
	}
	chunkIndex, before := ls.binarySearchForChunk(f)
	if !before.IsUnknown() {
		if before.IsTrue() {
			return chunkIndex * ls.opts.ChunkSize, false
		}
		return ls.len(), false
	}
	var offset int
	offset, exact = slices.BinarySearchFunc(ls.chunks[chunkIndex], f, ls.binaryCompareFunc)
	i = chunkIndex*ls.opts.ChunkSize + offset
	// zlog.Warn("BS:", i, exact, offset, f)
	return i, exact
}

// Add adds t to the end of the linked slice using add().
// If DelaySortAddSecs != 0, it adds it to a slice with current time, and then calls
// setDelayedItems to sort delayed ones before adding with add().
func (ls *LinkedSlice[T, F]) Add(t T, auxData any) {
	ls.lock.Lock()
	defer ls.lock.Unlock()
	if ls.opts.DelaySortAddSecs != 0 {
		da := delayedAdd[T]{item: t, added: time.Now(), auxData: auxData}
		ls.delayedAdds = append(ls.delayedAdds, da)
		return
	}
	ls.add(t, auxData)
}

// add keeps adding t's to the last chunk, assuming ls is locked.
// If full before add, it forces a save on the chunk (if DB set), and increments setDelayedItems
// before adding a new chunk and adding.
func (ls *LinkedSlice[T, F]) add(t T, auxData any) {
	zlog.Assert(auxData == nil || ls.GetIDFunc != nil, auxData == nil, ls.GetIDFunc == nil)
	count := len(ls.chunks)
	if count == 0 || len(ls.chunks[count-1]) == ls.opts.ChunkSize {
		if ls.opts.DB != nil {
			if ls.touched {
				ls.saveLastChunk() // we don't clear ls.touched, as we add to it after
			}
			ls.currentChunkSaveIndex++
		}
		c := make([]T, 1, ls.opts.ChunkSize)
		c[0] = t
		ls.chunks = append(ls.chunks, c)
		return
	}
	if auxData != nil {
		key := makeKey(auxDataKeyPrefix, ls.GetIDFunc(t))
		ls.auxToSaveWithChunk[key] = auxData
	}
	ls.chunks[count-1] = append(ls.chunks[count-1], t)
	ls.touched = true
}

func (ls *LinkedSlice[T, F]) SaveLastChunk() error {
	ls.lock.Lock()
	err := ls.saveLastChunk()
	ls.lock.Unlock()
	return err
}

func (ls *LinkedSlice[T, F]) saveLastChunk() error {
	if len(ls.chunks) == 0 {
		return nil
	}
	chunk := &ls.chunks[len(ls.chunks)-1]

	batch := ls.opts.DB.NewBatch(lotusdb.DefaultBatchOptions)
	ibytes := zbytes.Int64ToBytes(uint64(ls.currentChunkSaveIndex))
	err := batch.Put([]byte(currentChunkIndexKey), ibytes)
	if zlog.OnError(err, currentChunkIndexKey) {
		return err
	}
	chunkKey := makeKey(chunkKeyPrefix, int64(ls.currentChunkSaveIndex))
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err = enc.Encode(chunk)
	zlog.AssertNotError(err)
	err = batch.Put([]byte(chunkKey), buff.Bytes())
	if zlog.OnError(err, chunkKey) {
		return err
	}
	for k, v := range ls.auxToSaveWithChunk {
		var buff bytes.Buffer
		enc := gob.NewEncoder(&buff)
		err := enc.Encode(v)
		err = batch.Put([]byte(k), buff.Bytes())
		if zlog.OnError(err, k) {
			return err
		}
	}
	ls.auxToSaveWithChunk = map[string]any{}
	err = batch.Commit()
	if zlog.OnError(err) {
		return err
	}
	return nil
}

func (ls *LinkedSlice[T, F]) GetInt64ForKeyFromDB(key string) (int64, bool, error) {
	data, err := ls.opts.DB.Get([]byte(key))
	if err == nil {
		return int64(zbytes.BytesToInt64(data)), true, nil
	}
	if err == lotusdb.ErrKeyNotFound {
		return 0, false, nil
	}
	return 0, false, zlog.Error(err, "•", "key:", key)
}

func (ls *LinkedSlice[T, F]) SetInt64ForKeyToDB(key string, v int64) error {
	data := zbytes.Int64ToBytes(uint64(v))
	err := ls.opts.DB.Put([]byte(key), data)
	if zlog.OnError(err) {
		return err
	}
	return nil
}

func (ls *LinkedSlice[T, F]) Load() error {
	zlog.Assert(ls.opts.DB != nil, "has db")
	ls.lock.Lock()
	index, got, err := ls.GetInt64ForKeyFromDB(currentChunkIndexKey)
	if err != nil {
		ls.lock.Unlock()
		return zlog.Error(err)
	}
	if !got {
		ls.Loaded = true
		ls.lock.Unlock()
		return nil
	}
	ls.currentChunkSaveIndex = int(index)
	if !ls.loadChunk(ls.currentChunkSaveIndex) {
		ls.Loaded = true
		ls.lock.Unlock()
		return zlog.Error("Error loading first chunk", ls.currentChunkSaveIndex)
	}
	// we load top chunk so we can get going
	from := ls.currentChunkSaveIndex - 1
	ls.lock.Unlock()
	go func(from int) { // then do rest in a go routine
		for i := from; i >= 0; i-- {
			if !ls.loadChunk(i) {
				break
			}
		}
		ls.Loaded = true
	}(from)
	return nil
}

func (ls *LinkedSlice[T, F]) loadChunk(chunkSaveIndex int) bool {
	chunkKey := makeKey(chunkKeyPrefix, int64(chunkSaveIndex))
	data, err := ls.opts.DB.Get([]byte(chunkKey))
	if err != nil {
		if err != lotusdb.ErrKeyNotFound {
			zlog.Error(err, "index:", chunkSaveIndex)
		}
		ls.Loaded = true
		return false
	}
	buff := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buff)
	var chunk []T
	err = dec.Decode(&chunk)
	zlog.AssertNotError(err)
	ls.lock.Lock()
	ls.chunks = append([][]T{chunk}, ls.chunks...)
	ls.lock.Unlock()
	// zlog.Warn("Loaded:", zlog.Full(ls.chunks))
	return true
}

type delayedAdd[T any] struct {
	item    T
	added   time.Time
	auxData any
}

const (
	currentChunkIndexKey = "curchix"
	chunkKeyPrefix       = "chnk"
	auxDataKeyPrefix     = "aux"
)

func makeKey(prefix string, n int64) string {
	return fmt.Sprintf("%s%d", prefix, n)
}

func (ls *LinkedSlice[T, F]) GetAuxData(id int64, dataPtr any) error {
	key := makeKey(auxDataKeyPrefix, id)
	data, err := ls.opts.DB.Get([]byte(key))
	if err != nil {
		return zlog.Error(err, key)
	}
	buff := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buff)
	err = dec.Decode(dataPtr)
	if err != nil {
		return zlog.Error(err, key, len(data))
	}
	return nil
}
