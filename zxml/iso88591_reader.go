package uxml

import (
	"bytes"
	"errors"
	"io"
	"unicode/utf8"
)

type ISO88591Reader struct {
	r   io.ByteReader
	buf *bytes.Buffer
}

func NewISO88591Reader(r io.Reader) *ISO88591Reader {
	buf := bytes.NewBuffer(make([]byte, 0, utf8.UTFMax))
	return &ISO88591Reader{r.(io.ByteReader), buf}
}

func (cs *ISO88591Reader) ReadByte() (b byte, err error) {
	// http://unicode.org/Public/MAPPINGS/ISO8859/8859-1.TXT
	// Date: 1999 July 27; Last modified: 27-Feb-2001 05:08
	if cs.buf.Len() <= 0 {
		r, err := cs.r.ReadByte()
		if err != nil {
			return 0, err
		}
		if r < utf8.RuneSelf {
			return r, nil
		}
		cs.buf.WriteRune(rune(r))
	}
	return cs.buf.ReadByte()
}

func (cs *ISO88591Reader) Read(p []byte) (int, error) {
	// Use ReadByte method.
	return 0, errors.New("skip")
}
