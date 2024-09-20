//go:build server

package zchunkedrows

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type CRCommander struct {
	lastChunkIndex int
	lastIndex      int
	ChunkedRows    *ChunkedRows
	OrdererIsTime  bool
	otherColumns   []termColumn // this is map of offsets to int columns. For showing in rows.
	shownRows      bool

	UpdateTermColumnsFunc func()
}

type termColumn struct {
	offset  int
	header  string
	is32Bit bool
	isMask  bool
	isTime  bool
	names   map[int]string
}

func (crc *CRCommander) SetTermColumn(offset int, header string, is32Bit, isMask, isTime bool, names map[int]string) {
	col := termColumn{offset: offset, header: header, is32Bit: is32Bit, isMask: isMask, isTime: isTime, names: names}
	zslice.AddOrReplace(&crc.otherColumns, col, func(a, b termColumn) bool {
		return a.offset == b.offset
	})
}

func (crc *CRCommander) Rows(c *zcommands.CommandInfo, a struct {
	ChunkIndex *int    `zui:"title:ci,desc:Start Chunk"`
	Index      *int    `zui:"title:i,desc:Start Index"`
	Match      *string `zui:"title:m,desc:Text to match rows with"`
}) string {
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "lists rows in the table"
	}
	if !crc.shownRows {
		crc.lastChunkIndex = -1
		crc.lastIndex = -1
		crc.shownRows = true
	}
	if a.ChunkIndex != nil {
		crc.lastChunkIndex = *a.ChunkIndex
		crc.lastIndex = 0
		if *a.ChunkIndex == -1 {
			crc.lastIndex = -1
		}
	}
	if a.Index != nil {
		crc.lastIndex = *a.Index
	}
	var match string
	if a.Match != nil {
		match = *a.Match
	}
	if crc.UpdateTermColumnsFunc != nil {
		crc.UpdateTermColumnsFunc()
	}
	zlog.Warn("Rows:", a.ChunkIndex != nil, a.Index != nil, crc.lastChunkIndex, crc.lastIndex, match)
	outputRows(crc, c, match)
	return ""
}

func outputRows(crc *CRCommander, c *zcommands.CommandInfo, match string) {
	w := c.Session.TermSession.Writer()
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 60

	zlog.Warn("OutRows:", zlog.Full(crc.otherColumns))
	fmt.Fprint(tabs, zstr.EscGreen, "chunk\tindex\t")
	if crc.ChunkedRows.opts.HasIncreasingIDFirstInRow {
		fmt.Fprint(tabs, "id\t")
	}
	if crc.ChunkedRows.opts.OrdererOffset != 0 {
		if crc.OrdererIsTime {
			fmt.Fprint(tabs, "time\t")
		} else {
			fmt.Fprint(tabs, "orderer\t")
		}
	}
	for _, col := range crc.otherColumns {
		fmt.Fprint(tabs, col.header, "\t")
	}
	if crc.ChunkedRows.opts.MatchIndexOffset != 0 {
		fmt.Fprint(tabs, "text\t")
	}
	fmt.Fprint(tabs, zstr.EscNoColor, "\n")
	i := 0
	err := crc.ChunkedRows.Iterate(crc.lastChunkIndex, crc.lastIndex, false, match, func(row []byte, chunkIndex, index int) bool {
		crc.outputRow(c, tabs, row, chunkIndex, index)
		i++
		crc.lastChunkIndex = chunkIndex
		crc.lastIndex = index
		return i < 40
	})
	tabs.Flush()
	zlog.Warn("DidRows:", crc.lastChunkIndex, crc.lastIndex, match)
	if err != nil {
		fmt.Fprintln(tabs, zstr.EscMagenta, err, zstr.EscNoColor)
		return
	}
}

func (crc *CRCommander) outputRow(c *zcommands.CommandInfo, tabs *zstr.TabWriter, row []byte, chunkIndex, index int) {
	fmt.Fprint(tabs, chunkIndex, "\t", index, "\t")
	if crc.ChunkedRows.opts.HasIncreasingIDFirstInRow {
		id := int64(binary.LittleEndian.Uint64(row[0:]))
		fmt.Fprint(tabs, id, "\t")
	}
	if crc.ChunkedRows.opts.OrdererOffset != 0 {
		o := int64(binary.LittleEndian.Uint64(row[crc.ChunkedRows.opts.OrdererOffset:]))
		if crc.OrdererIsTime {
			t := time.UnixMicro(o)
			fmt.Fprint(tabs, ztime.GetNiceSubSecs(t, true, 3), "\t")
		} else {
			fmt.Fprint(tabs, o, "\t")
		}
	}
	for _, col := range crc.otherColumns {
		var n int64
		if col.is32Bit {
			n = int64(binary.LittleEndian.Uint32(row[col.offset:]))
		} else {
			n = int64(binary.LittleEndian.Uint64(row[col.offset:]))
		}
		if col.isTime {
			t := time.UnixMicro(n)
			fmt.Fprint(tabs, ztime.GetNiceSubSecs(t, true, 3), "\t")
			continue
		}
		if len(col.names) != 0 {
			name := col.names[int(n)]
			if name != "" {
				fmt.Fprint(tabs, name, "\t")
				continue
			}
		}
		if n == 0 {
			fmt.Fprint(tabs, "x\t")
			continue
		}
		fmt.Fprint(tabs, n, "\t")
	}
	if crc.ChunkedRows.opts.MatchIndexOffset != 0 {
		match, err := crc.ChunkedRows.getMatchStr(chunkIndex, row)
		if err != nil {
			match = err.Error()
		}
		fmt.Fprint(tabs, match, "\t")
	}
	fmt.Fprint(tabs, "\n")
}

func (crc *CRCommander) Info(c *zcommands.CommandInfo) string {
	// zlog.Warn("CRCommander.Info:", zlog.Pointer(crc.ChunkedRows))
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "Show information about the chunked rows."
	}
	w := c.Session.TermSession.Writer()
	dict := zdict.FromStruct(crc.ChunkedRows.opts, false)
	dict["BottomChunkIndex"] = crc.ChunkedRows.bottomChunkIndex
	dict["TopChunkIndex"] = crc.ChunkedRows.topChunkIndex
	dict["TopChunkRowCount"] = crc.ChunkedRows.topChunkRowCount
	dict["CurrentID"] = crc.ChunkedRows.currentID
	dict["TotalRows"] = crc.ChunkedRows.TotalRowCount()
	dict.WriteTabulated(w)

	return ""
}
