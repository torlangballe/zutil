package uxml

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"strings"

	"github.com/torlangballe/zutil/zbytes"
)

func isCharsetISO88591(charset string) bool {
	// http://www.iana.org/assignments/character-sets
	switch strings.ToLower(charset) {
	case "iso_8859-1:1987":
		return true
	case "iso-8859-1":
		return true
	case "iso-ir-100":
		return true
	case "iso_8859-1":
		return true
	case "latin1":
		return true
	case "l1":
		return true
	case "ibm819":
		return true
	case "cp819":
		return true
	case "csisolatin1":
		return true
	}
	return false
}

func isCharsetUTF8(charset string) bool {
	// FIXME: Incredible ugly/wrong 'fix' to get rid of Amedia RSS errors.
	if (charset == "UTF-8") || (charset == "UTF-16") {
		return true
	}
	return false
}

func CharsetReader(charset string, input io.Reader) (reader io.Reader, err error) {
	switch {
	case isCharsetUTF8(charset):
		reader = NewValidUTF8Reader(input)

	case isCharsetISO88591(charset):
		reader = NewISO88591Reader(input)

	default:
		err = errors.New("CharsetReader: unexpected charset: " + charset)
	}

	return
}

func EscapeStringForXML(str string) string {
	var buf bytes.Buffer // A Buffer needs no initialization.
	writer := bufio.NewWriter(&buf)
	xml.EscapeText(writer, []byte(str))
	writer.Flush()
	return buf.String()
}

func identReader(encoding string, input io.Reader) (io.Reader, error) {
	return input, nil
}

func UnmarshalUTF16(b []byte, target interface{}) error {
	reader := zbytes.MakeUTF16Reader(b)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = identReader
	return decoder.Decode(target)
}

func UnmarshalWithBOM(b []byte, target interface{}) error {
	if zbytes.HasUnicodeBOM(b) {
		reader := zbytes.MakeUTF16Reader(b)
		decoder := xml.NewDecoder(reader)
		decoder.CharsetReader = identReader
		return decoder.Decode(target)
	}
	return xml.Unmarshal(b, target)
}
