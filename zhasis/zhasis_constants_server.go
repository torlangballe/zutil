//go:build server

package zhasis

import (
	"encoding/json"
	"fmt"

	"github.com/torlangballe/zutil/zlog"
)

func initConstants() {
	query := `
	CREATE TABLE IF NOT EXISTS constants (
		id SERIAL PRIMARY KEY,
		constant TEXT NOT NULL DEFAULT '',
		UNIQUE (constant)
	)`
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
}

func selectConstants(where string) (map[int64]string, error) {
	m := map[int64]string{}
	query := "SELECT id, constant FROM constants WHERE " + where
	rows, err := database.Query(query)
	if err != nil {
		return nil, zlog.Error(err, query)
	}
	for rows.Next() {
		var id int64
		var str string
		err = rows.Scan(&id, &str)
		if err != nil {
			return nil, zlog.Error(err, query)
		}
		m[id] = str
	}
	return m, nil
}

func ConstantRawStr(id int64) (string, error) {
	var str string
	row := database.QueryRow("SELECT constant FROM constants WHERE id=$1", id)
	err := row.Scan(&str)
	if err != nil {
		return "", err
	}
	return str, nil
}

func ConstantAsString(id int64) (string, error) {
	str, err := ConstantRawStr(id)
	if err != nil {
		return "", err
	}
	return str, nil
}

func ConstantToObject(id int64, objectPtr any) error {
	str, err := ConstantRawStr(id)
	if err != nil {
		return err
	}
	err = json.Unmarshal([]byte(str), objectPtr)
	return err
}

func ConstantOnly(id int64) string {
	str, err := ConstantAsString(id)
	if err != nil {
		str = "<err:" + err.Error() + ">"
	}
	return str
}

func constantInfo(id int64) string {
	str := ConstantOnly(id)
	if err != nil {
		str = "<err:" + err.Error() + ">"
	}
	return fmt.Sprintf("%d*%s", id, str)
}
