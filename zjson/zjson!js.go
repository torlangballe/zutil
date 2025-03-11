//go:build !js

package zjson

import (
	"encoding/json"
	"io"
	"os"

	"github.com/torlangballe/zutil/zfile"
)

func UnmarshalFromFile(to any, fpath string, allowNoFile bool) error {
	if allowNoFile && zfile.NotExists(fpath) {
		return nil
	}
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

// MarshalToFile marshals from into a json byte stream that is written to fpath.
// It happens atomically using a temporary file
func MarshalToFile(from any, fpath string) error {
	return zfile.WriteToFileAtomically(fpath, func(file io.Writer) error {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("  ", "  ")
		return encoder.Encode(from)
	})
}
