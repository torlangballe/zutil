package zstr

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/torlangballe/zutil/zint"
)

// TabWriter implements an io.Writer that reads text and outputs it in columns to a writer,
// splitting the output using \t and \n for each ine
// TODO: Currenly uses a hack to allow multi-lines per cell using \r.
// Need a better way, but might need to specify number of columns.
// It can handle terminal color escape codes.

type TabWriter struct {
	MaxColumnWidth      int          // MaxColumnWidth is a global max width for any column not otherwise specified
	OutCellDivider      string       // OutCellDivider is what to have between columns
	RighAdjustedColumns map[int]bool // Set RighAdjustedColumns for a zero-based index of column to right adjust
	MaxColumnWidths     map[int]int  // MaxColumnWidths allows user to specify a max per-column
	RepeatFirstRowEvery int          // Allows the first row (header) to be repeated every so often
	firstLine           string
	buffer              bytes.Buffer
	out                 io.Writer
}

func NewTabWriter(out io.Writer) *TabWriter {
	t := &TabWriter{}
	t.out = out
	t.OutCellDivider = "  "
	t.RighAdjustedColumns = map[int]bool{}
	t.MaxColumnWidths = map[int]int{}
	t.RepeatFirstRowEvery = 20
	return t
}

func (t *TabWriter) Write(b []byte) (n int, err error) {
	return t.buffer.Write(b)
}

func (t *TabWriter) Flush() error {
	var widths []int
	allOutput := string(t.buffer.Bytes())
	lines := strings.Split(allOutput, "\n")
	for _, sline := range lines {
		sline = strings.TrimRight(sline, "\t")
		for i, cell := range strings.Split(sline, "\t") {
			if len(widths)-1 < i {
				widths = append(widths, 0)
			}
			stripped := ReplaceAllCapturesFunc(colorEscapeReg, cell, 0, func(cap string, index int) string {
				return ""
			})
			clen := utf8.RuneCountInString(stripped)
			zint.Maximize(&widths[i], clen)
		}
	}
	for i := range widths {
		max := t.MaxColumnWidths[i]
		if max == 0 {
			max = t.MaxColumnWidth
		}
		if max != 0 {
			zint.Minimize(&widths[i], max)
		}
	}
	// fmt.Println("Widths:", widths)
	var lineIndex int
	for _, sline := range lines {
		sline = strings.TrimRight(sline, "\t")
		if t.RepeatFirstRowEvery != 0 {
			if lineIndex == 0 {
				t.firstLine = sline
			} else if lineIndex >= t.RepeatFirstRowEvery && (lineIndex-1)%t.RepeatFirstRowEvery == 0 {
				t.outputLine(t.firstLine, widths)
			}
		}
		t.outputLine(sline, widths)
		lineIndex++
	}
	return nil
}

func (t *TabWriter) outputLine(sline string, widths []int) {
	cells := strings.Split(sline, "\t")
	for j := 0; ; j++ {
		var has bool
		row := make([]string, len(cells))
		for i, c := range cells {
			parts := strings.Split(c, "\r")
			if len(parts) > j {
				has = true
				row[i] = parts[j]
			}
		}
		if has {
			t.outputCells(row, widths)
		} else {
			break
		}
	}
}

func (t *TabWriter) outputCells(cells []string, widths []int) {
	var outLine string
	for i, cell := range cells {
		var cellChars, whiteChars int
		width := widths[i]
		shortened := ReplaceAllCapturesFunc(colorEscapeReg, cell, RegWithOutside, func(cap string, index int) string {
			if index < 0 { // it's not the escape char
				left := width - cellChars
				if left <= 0 {
					return ""
				}
				capLen := utf8.RuneCountInString(cap)
				if left < capLen {
					whiteChars += left
					cap = string([]rune(cap)[:left])
				} else {
					whiteChars += capLen
				}
				cellChars += capLen
			}
			return cap
		})
		if i != 0 {
			outLine += t.OutCellDivider
		}
		left := width - whiteChars
		var space string
		if left > 0 {
			space = strings.Repeat(" ", left)
		}
		right := t.RighAdjustedColumns[i]
		if right {
			outLine += space + shortened
		} else {
			outLine += shortened + space
		}
	}
	t.out.Write([]byte(outLine + "\n"))
}

var colorEscapeReg = regexp.MustCompile(`([\x{1B}]\[[0-9;]+m)`)
