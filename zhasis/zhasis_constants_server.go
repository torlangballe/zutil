//go:build server

package zhasis

import (
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

func constantString(id int64) string {
	var str string
	row := database.QueryRow("SELECT constant FROM constants WHERE id=$1", id)
	err := row.Scan(&str)
	if err != nil {
		return "<err:" + err.Error() + ">"
	}
	return str
}

func constantInfo(id int64) string {
	return fmt.Sprintf("%d*%s", id, constantString(id))
}
