package zjson

import (
	"encoding/json"
	"io"
	"os"

	"github.com/torlangballe/zutil/zfile"
)

func UnmarshalFromFile(to interface{}, fpath string) error {
	file, err := os.Open(fpath)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(to)
	if err != nil {
		return err
	}
	return nil
}

// MarshalToFile marshals from into a json byte stream that is writted to fpath.
// It happens atomically using a temporary file
func MarshalToFile(from interface{}, fpath string) error {
	return zfile.WriteToFileAtomically(fpath, func(file io.Writer) error {
		encoder := json.NewEncoder(file)
		return encoder.Encode(from)
	})
}
