//go:build server

package zchunkedrows

import (
	"encoding/binary"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type CRCommander struct {
	lastChunkIndex int
	lastIndex      int
	ChunkedRows    *ChunkedRows
	OrdererIsTime  bool
	// RowParameters    zfields.FieldParameters
	// EditParameters   zfields.FieldParameters
}

//const RowUseZTermSliceName = "$zterm"

// func makeRowParameters() zfields.FieldParameters {
// 	var p zfields.FieldParameters

// 	p.UseInValues = []string{RowUseZTermSliceName}
// 	return p
// }

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
	if a.ChunkIndex != nil {
		crc.lastChunkIndex = *a.ChunkIndex
		crc.lastIndex = 0
	}
	if a.Index != nil {
		crc.lastIndex = *a.Index
	}
	var match string
	if a.Match != nil {
		match = *a.Match
	}
	zlog.Warn("Rows:", a.ChunkIndex != nil, a.Index != nil, crc.lastChunkIndex, crc.lastIndex, match)
	outputRows(crc, c, match)
	return ""
}

func outputRows(crc *CRCommander, c *zcommands.CommandInfo, match string) {
	var hid, horderer, hmatch string
	w := c.Session.TermSession.Writer()
	tabs := zstr.NewTabWriter(w)
	tabs.MaxColumnWidth = 60

	if crc.ChunkedRows.opts.HasIncreasingIDFirstInRow {
		hid = "id\t"
	}
	if crc.ChunkedRows.opts.OrdererOffset != 0 {
		horderer = "orderer\t"
		if crc.OrdererIsTime {
			horderer = "time\t"
		}
	}
	if crc.ChunkedRows.opts.MatchIndexOffset != 0 {
		hmatch = "text\t"
	}
	fmt.Fprint(tabs, zstr.EscGreen, "chunk\tindex\t", hid, horderer, hmatch, zstr.EscNoColor, "\n")
	i := 0
	err := crc.ChunkedRows.Iterate(crc.lastChunkIndex, crc.lastIndex, true, match, func(row []byte, chunkIndex, index int) bool {
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
