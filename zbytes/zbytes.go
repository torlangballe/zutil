package zbytes

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"io"
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
