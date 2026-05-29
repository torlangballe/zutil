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

// RollingBuffer is a buffer that keeps only the last N bytes of data written to it.
// It implements io.Writer, io.Reader and io.Seeker interfaces.
// It can be used for keeping logs in memory with a maximum size.
// It never returns EOF on read, but returns 0, nil when there is no data to read. Yet.
type RollingBuffer struct {
	data           []byte
	currentReadPos int
	maxSize        int
}

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

func UInt64ToBytes(n uint64) []byte {
	var c [8]byte
	binary.LittleEndian.PutUint64(c[0:], n)
	return c[0:]
}

func BytesToUInt64(b []byte) uint64 {
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

func NewRollingBuffer(maxSize int) *RollingBuffer {
	return &RollingBuffer{
		maxSize: maxSize,
	}
}

func (r *RollingBuffer) Write(p []byte) (n int, err error) {
	r.data = append(r.data, p...)
	diff := len(r.data) - r.maxSize
	if diff > 0 {
		r.data = r.data[diff:]
		if r.currentReadPos > diff {
			r.currentReadPos -= diff
		} else {
			r.currentReadPos = 0
		}
	}
	return len(p), nil
}

func (r *RollingBuffer) Seek(offset int64, whence int) (int64, error) {
	dlen := len(r.data)
	switch whence {
	case io.SeekStart:
		if offset < 0 || offset > int64(dlen) {
			return 0, io.ErrUnexpectedEOF
		}
		r.currentReadPos = int(offset)
	case io.SeekCurrent:
		if r.currentReadPos+int(offset) < 0 || r.currentReadPos+int(offset) > int(dlen) {
			return 0, io.ErrUnexpectedEOF
		}
		r.currentReadPos += int(offset)
	case io.SeekEnd:
		if int64(dlen)-offset < 0 || int64(dlen)-offset > int64(dlen) {
			return 0, io.ErrUnexpectedEOF
		}
		r.currentReadPos = dlen - int(offset)
	}
	return int64(r.currentReadPos), nil
}

func (r *RollingBuffer) Read(p []byte) (n int, err error) {
	dlen := len(r.data)
	if dlen == 0 || r.currentReadPos >= dlen {
		return 0, nil
	}
	diff := dlen - r.currentReadPos
	get := min(diff, len(p))
	copy(p, r.data[r.currentReadPos:r.currentReadPos+get])
	r.currentReadPos += get
	return int(get), nil
}

