package zjson

import (
	"encoding/json"
	"os"

	"github.com/torlangballe/zutil/zlog"
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

func MarshalToFile(from interface{}, fpath string) error {
	file, err := os.Create(fpath)
	if err != nil {
		return nil
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	err = encoder.Encode(from)
	if err != nil {
		return zlog.Error(err, "marshal", from)
	}
	return nil
}
