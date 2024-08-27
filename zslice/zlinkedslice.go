package zslice

import (
	"slices"

	"github.com/torlangballe/zutil/zbool"
)

type LinkedSlice[S ~[]T, T, F any] struct {
	chunkSize   int
	chunks      []S
	compareFunc func(T, F) int
}

func NewLinked[S ~[]T, T, F any](chunkSize int, compare func(T, F) int) *LinkedSlice[S, T, F] {
	var ls LinkedSlice[S, T, F]
	if chunkSize == 0 {
		chunkSize = 100 * 1024
	}
	ls.chunkSize = chunkSize
	ls.compareFunc = compare
	return &ls
}

func (ls *LinkedSlice[S, T, F]) Len() int {
	count := len(ls.chunks)
	if count == 0 {
		return 0
	}
	return (count-1)*ls.chunkSize + len(ls.chunks[count-1])
}

func (ls *LinkedSlice[S, T, F]) Index(i int) *T {
	ci := i / ls.chunkSize
	si := i % ls.chunkSize
	return &ls.chunks[ci][si]
}

func (ls *LinkedSlice[S, T, F]) BinarySearchForChunk(f F) (i int, before zbool.BoolInd) {
	lsLen := len(ls.chunks)
	if lsLen == 0 {
		return 0, zbool.Unknown
	}
	for i, c := range ls.chunks {
		if ls.compareFunc(c[0], f) > 0 {
			return i, zbool.True
		}
		comp := ls.compareFunc(c[len(c)-1], f)
		if comp < 0 {
			continue
		}
		return i, zbool.Unknown
	}
	// fmt.Println("BinarySearchForChunk at end", f, lsLen-1)
	return lsLen - 1, zbool.False
}

func (ls *LinkedSlice[S, T, F]) BinarySearch(f F) (i int, exact bool) {
	chunksLen := len(ls.chunks)
	if chunksLen == 0 {
		return 0, false
	}
	chunkIndex, before := ls.BinarySearchForChunk(f)
	if !before.IsUnknown() {
		if before.IsTrue() {
			return chunkIndex * ls.chunkSize, false
		}
		return ls.Len(), false
	}
	var offset int
	offset, exact = slices.BinarySearchFunc(ls.chunks[chunkIndex], f, ls.compareFunc)
	i = chunkIndex*ls.chunkSize + offset
	// zlog.Warn("BS:", i, exact, offset, f)
	return i, exact
}

func (ls *LinkedSlice[S, T, F]) Add(t T) (chunk int) {
	count := len(ls.chunks)
	if count == 0 || len(ls.chunks[count-1]) == ls.chunkSize {
		c := make(S, 1, ls.chunkSize)
		c[0] = t
		ls.chunks = append(ls.chunks, c)
		return count // not +1, it's index, not size
	}
	ls.chunks[count-1] = append(ls.chunks[count-1], t)
	return count - 1
}
