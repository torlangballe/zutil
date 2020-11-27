package zbytes

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
	"io/ioutil"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

func CompressObject(o interface{}) (out []byte, err error) {
	var buf bytes.Buffer

	rawVal, err := json.Marshal(o)
	if err != nil {
		return
	}
	compressor, err := zlib.NewWriterLevel(&buf, zlib.BestSpeed)
	if err != nil {
		return
	}
	compressor.Write(rawVal)
	compressor.Close()

	out = buf.Bytes()
	return
}

func DecompressToObject(o interface{}, b []byte) (err error) {
	buf := bytes.NewBuffer(b)
	var uncompressor io.ReadCloser
	uncompressor, err = zlib.NewReader(buf)
	if err != nil {
		return
	}
	defer uncompressor.Close()
	dec := json.NewDecoder(uncompressor)
	err = dec.Decode(o)

	return
}

func HasUnicodeBOM(b []byte) bool {
	if len(b) >= 2 {
		if b[0] == 0xff && b[1] == 0xfe {
			return true
		}
		if b[0] == 0xfe && b[1] == 0xff {
			return true
		}
	}
	return false
}

func RemoveUnicodeBOM(b []byte) ([]byte, bool) {
	if HasUnicodeBOM(b) {
		return b[2:], true
	}
	return b, false
}

func DecodeUTF16(b []byte) ([]byte, error) {
	unicodeReader := MakeUTF16Reader(b)
	decoded, err := ioutil.ReadAll(unicodeReader)
	if err != nil {
		return b, err
	}
	return decoded, nil
}

func MakeUTF16Reader(b []byte) io.Reader {
	win16be := unicode.UTF16(unicode.BigEndian, unicode.IgnoreBOM)
	utf16bom := unicode.BOMOverride(win16be.NewDecoder())
	return transform.NewReader(bytes.NewReader(b), utf16bom)
}

func DecodeUTF16IfBOM(b []byte) ([]byte, error) {
	if HasUnicodeBOM(b) {
		return DecodeUTF16(b)
	}
	return b, nil
}

