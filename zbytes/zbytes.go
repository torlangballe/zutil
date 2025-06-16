package zbytes

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"io/ioutil"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	KiloByte  = 1024
	MegaByte  = 1024 * KiloByte
	GigaByte  = 1024 * MegaByte
	TerraByte = 1024 * GigaByte
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

func Int64ToBytes(n uint64) []byte {
	var c [8]byte
	binary.LittleEndian.PutUint64(c[0:], n)
	return c[0:]
}

func BytesToInt64(b []byte) uint64 {
	return binary.LittleEndian.Uint64(b)
}

// CountReadUntilError reads reader until it is exhausted.
// if it stops by EOF, no error is returned.
// it is for draining  a reader just to count it.
// Can be used to get file size of a file system with only io.Reader access.
func CountReadUntilError(reader io.Reader) (int64, error) {
	var count int64
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		count += int64(n)
		if err != nil {
			if err == io.EOF {
				return count, nil
			}
			return count, err
		}
	}
}

func CalculateCRC32(data []byte, seed int64) int64 {
	if seed == 0 {
		seed = crc32.Castagnoli
	}
	buf := bytes.NewBuffer(data)
	crc := crc32.New(crc32.MakeTable(uint32(seed)))
	io.Copy(crc, buf)
	return int64(crc.Sum32())
}
