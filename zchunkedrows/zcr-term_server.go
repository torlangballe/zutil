//go:build server

package zchunkedrows

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type CRCommander struct {
	lastChunkIndex int
	lastIndex      int
	chunkedRows    *ChunkedRows
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

func (crc *CRCommander) SetChunkedRows(cr *ChunkedRows) {
	crc.chunkedRows = cr
}

func (crc *CRCommander) SetTermColumn(offset int, header string, is32Bit, isMask, isTime bool, names map[int]string) {
	col := termColumn{offset: offset, header: header, is32Bit: is32Bit, isMask: isMask, isTime: isTime, names: names}
	zslice.AddOrReplace(&crc.otherColumns, col, func(a, b termColumn) bool {
		return a.offset == b.offset
	})
}

func (crc *CRCommander) CC(c *zcommands.CommandInfo) string {
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "clear cursor for continues 'rows' command"
	}
	crc.lastChunkIndex = -1
	crc.lastIndex = -1
	c.Session.TermSession.SetPrompt("> ")
	return ""
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
	forward := true
	if a.ChunkIndex != nil {
		crc.lastChunkIndex = *a.ChunkIndex
		crc.lastIndex = 0
		if *a.ChunkIndex == -1 {
			forward = false
			crc.lastIndex = -1
		}
	} else if !crc.shownRows {
		crc.lastChunkIndex = -1
		crc.lastIndex = -1
		crc.shownRows = true
		forward = false
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
	w := c.Session.TermSession.Writer()
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 60

	start := time.Now()
	fmt.Fprint(tabs, zstr.EscGreen, "chunk\tindex\t")
	if crc.chunkedRows.opts.HasIncreasingIDFirstInRow {
		fmt.Fprint(tabs, "id\t")
	}
	if crc.chunkedRows.opts.OrdererOffset != 0 {
		if crc.OrdererIsTime {
			fmt.Fprint(tabs, "time\t")
		} else {
			fmt.Fprint(tabs, "orderer\t")
		}
	}
	for _, col := range crc.otherColumns {
		fmt.Fprint(tabs, col.header, "\t")
	}
	if crc.chunkedRows.opts.MatchIndexOffset != 0 {
		fmt.Fprint(tabs, "text\t")
	}
	fmt.Fprint(tabs, zstr.EscNoColor, "\n")
	i := 0
	totalRows, err := crc.chunkedRows.Iterate(crc.lastChunkIndex, crc.lastIndex, forward, match, func(row []byte, chunkIndex, index int) bool {
		crc.outputRow(c, tabs, row, chunkIndex, index)
		i++
		crc.lastChunkIndex = chunkIndex
		crc.lastIndex = index
		return i < 40
	})
	tabs.Flush()
	if err != nil {
		fmt.Fprintln(tabs, zstr.EscMagenta, err, zstr.EscNoColor)
		return ""
	}
	prompt := ""
	if crc.lastChunkIndex != -1 {
		prompt = fmt.Sprintf("cursor: %d:%d> ", crc.lastChunkIndex, crc.lastIndex)
	}
	c.Session.TermSession.SetPrompt(prompt)
	since := ztime.Since(start)
	if since > 2 {
		w := c.Session.TermSession.Writer()
		fmt.Fprintln(w, "duration:", zfloat.KeepFractionDigits(since, 1), "total rows:", zint.MakeHumanFriendly(totalRows))
	}
	return ""
}

func (crc *CRCommander) orderToString(row []byte) string {
	if crc.chunkedRows.opts.OrdererOffset == 0 {
		return ""
	}
	o := int64(binary.LittleEndian.Uint64(row[crc.chunkedRows.opts.OrdererOffset:]))
	if crc.OrdererIsTime {
		t := time.UnixMicro(o)
		return ztime.GetNiceSubSecs(t, true, 3) + "\t"
	}
	return fmt.Sprint(o, "\t")
}

func (crc *CRCommander) outputRow(c *zcommands.CommandInfo, tabs *zstr.TabWriter, row []byte, chunkIndex, index int) {
	fmt.Fprint(tabs, chunkIndex, "\t", index, "\t")
	if crc.chunkedRows.opts.HasIncreasingIDFirstInRow {
		id := int64(binary.LittleEndian.Uint64(row[0:]))
		fmt.Fprint(tabs, id, "\t")
	}
	fmt.Fprint(tabs, crc.orderToString(row))
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
			fmt.Fprint(tabs, "\t")
			continue
		}
		fmt.Fprint(tabs, n, "\t")
	}
	if crc.chunkedRows.opts.MatchIndexOffset != 0 {
		match, err := crc.chunkedRows.getMatchStr(chunkIndex, row)
		if err != nil {
			match = err.Error()
		}
		fmt.Fprint(tabs, match, "\t")
	}
	fmt.Fprint(tabs, "\n")
}

func (crc *CRCommander) Info(c *zcommands.CommandInfo) string {
	// zlog.Warn("CRCommander.Info:", zlog.Pointer(crc.chunkedRows))
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "Show information about the chunked rows."
	}
	zlog.Info("info!")
	w := c.Session.TermSession.Writer()
	dict := zdict.FromStruct(crc.chunkedRows.opts, false)
	zlog.Info("info2!")
	dict["BottomChunkIndex"] = crc.chunkedRows.bottomChunkIndex
	dict["TopChunkIndex"] = crc.chunkedRows.topChunkIndex
	dict["TopChunkRowCount"] = crc.chunkedRows.topChunkRowCount
	dict["CurrentID"] = crc.chunkedRows.currentID
	zlog.Info("info3!")
	dict["TotalRows"] = crc.chunkedRows.TotalRowCount()
	zlog.Info("info4!")
	dict.WriteTabulated(w)

	return ""
}

func (crc *CRCommander) Chunks(c *zcommands.CommandInfo) string {
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "Show details about each chunk."
	}
	w := c.Session.TermSession.Writer()
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 60

	cr := crc.chunkedRows
	fmt.Fprintln(tabs, zstr.EscGreen+"chunk\tfilelen\tfilerows\trows\tfirstid\tlastid\tiddiff", zstr.EscNoColor)
	for i := cr.bottomChunkIndex; i <= cr.topChunkIndex; i++ {
		var idStart, idEnd int64
		fpath := cr.chunkFilepath(i, isRows)
		size := zfile.Size(fpath)
		rcf := float64(size) / float64(cr.opts.RowByteSize)
		rci := int(rcf)
		fmt.Fprint(tabs, i, "\t", zint.MakeHumanFriendly(size), "\t")
		_, fract := math.Modf(rcf)
		if fract != 0.0 {
			fmt.Fprint(tabs, zstr.EscRed, rcf, zstr.EscNoColor, "\t")
		} else if rci != cr.opts.RowsPerChunk && i != cr.topChunkIndex {
			fmt.Fprint(tabs, zstr.EscRed, zint.MakeHumanFriendly(rci), zstr.EscNoColor, "\t")
		} else {
			fmt.Fprint(tabs, zint.MakeHumanFriendly(rci), "\t")
		}
		fmt.Fprint(tabs, zint.MakeHumanFriendly(cr.opts.RowsPerChunk), "\t")
		mm, err := cr.getMemoryMap(i, isRows)
		if err != nil {
			fmt.Fprint(tabs, err, "\t")
		} else {
			row := make([]byte, cr.opts.RowByteSize)
			err = cr.readRow(0, row, mm)
			if err != nil {
				fmt.Fprint(tabs, err, "\t")
			} else {
				idStart = int64(binary.LittleEndian.Uint64(row[0:]))
				fmt.Fprint(tabs, idStart, "\t")
				err = cr.readRow(rci-1, row, mm)
				if err != nil {
					fmt.Fprint(tabs, err, "\t")
				} else {
					idEnd = int64(binary.LittleEndian.Uint64(row[0:]))
					fmt.Fprint(tabs, idEnd, "\t")
				}
			}
		}
		var col string
		idDiff := idEnd - idStart + 1
		if idDiff != int64(rci) {
			col = zstr.EscRed
		}
		fmt.Fprint(tabs, col, zint.MakeHumanFriendly(idDiff), zstr.EscNoColor, "\t")
		fmt.Fprint(tabs, "\n")
	}
	tabs.Flush()
	return ""
}

func (crc *CRCommander) Chunk(c *zcommands.CommandInfo, a struct {
	ChunkIndex int `zui:"title:ChunkIndex,desc:Chunk to show details of"`
}) string {
	switch c.Type {
	case zcommands.CommandExpand:
		return ""
	case zcommands.CommandHelp:
		return "Show details about a specific chunk, including missing ids."
	}
	w := c.Session.TermSession.Writer()

	cr := crc.chunkedRows
	if a.ChunkIndex < cr.bottomChunkIndex || a.ChunkIndex > cr.topChunkIndex {
		fmt.Fprintln(w, "Chunk Index of of bounds:", a.ChunkIndex, "[", cr.bottomChunkIndex, "-", cr.topChunkIndex, "]")
		return ""
	}
	fpath := cr.chunkFilepath(a.ChunkIndex, isRows)
	size := zfile.Size(fpath)
	rows := int(size) / cr.opts.RowByteSize
	fmt.Fprintln(w, "Chunk:", a.ChunkIndex, "Rows:", rows)
	lastID := int64(-1)
	mm, err := cr.getMemoryMap(a.ChunkIndex, isRows)
	if err != nil {
		fmt.Fprintln(w, "Error getting memory map:", err)
	}
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 60
	fmt.Fprintln(tabs, zstr.EscGreen+"row\ttime\tid\tstatus", zstr.EscNoColor)
	var prevOKLine string
	for i := 0; i < rows; i++ {
		var line string
		row := make([]byte, cr.opts.RowByteSize)
		line += fmt.Sprint(i, "\t")
		err = cr.readRow(i, row, mm)
		if err != nil {
			fmt.Fprint(tabs, line, err, "\t\t\t\n")
			continue
		}
		line += crc.orderToString(row)
		id := int64(binary.LittleEndian.Uint64(row[0:]))
		line += fmt.Sprint(id, "\t")
		lID := lastID
		lastID = id
		var serr string
		if id == lID {
			serr = "duplcate"
		} else if id < lID {
			serr = "below"
		} else if lID != -1 && id > lID+1 {
			serr = fmt.Sprint("skipped: ", id-lID-1)
		}
		if serr != "" {
			if prevOKLine != "" {
				fmt.Fprint(tabs, prevOKLine, "\n")
				prevOKLine = ""
			}
			fmt.Fprint(tabs, line, zstr.EscRed, serr, zstr.EscNoColor, "\n")
		} else {
			if i == 0 || i == rows-1 {
				fmt.Fprint(tabs, line, "\n")
				prevOKLine = ""
			} else {
				prevOKLine = line
			}
		}
	}
	tabs.Flush()
	return ""
}
