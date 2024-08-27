package zslice

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

func checkBinarySearchForChunk(t *testing.T, slice *LinkedSlice[[]int, int, int], val, wantChunkIndex int, before zbool.BoolInd) {
	index, bef := slice.BinarySearchForChunk(val)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearchForChunk %d:", val), index, wantChunkIndex)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearchForChunk %d before:", val), bef, before)
}

func checkBinarySearch(t *testing.T, slice *LinkedSlice[[]int, int, int], val, wantIndex int, exact bool) {
	index, ex := slice.BinarySearch(val)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearch %d:", val), index, wantIndex)
	ztesting.Compare(t, fmt.Sprintf("checkBinarySearch %d accurate:", val), ex, exact)
}

func TestAdd(t *testing.T) {
	zlog.Warn("TestAdd")
	slice := NewLinked[[]int, int, int](5, func(comp int, to int) int {
		return comp - to
	})
	for i := 1; i < 35; i += 3 {
		slice.Add(i)
	}
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
