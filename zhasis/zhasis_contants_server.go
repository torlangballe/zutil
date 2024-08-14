//go:build server

package zhasis

import (
	"github.com/torlangballe/zutil/zlog"
)

func initConstants() {
	query := `
	CREATE TABLE IF NOT EXISTS constants (
		id SERIAL PRIMARY KEY,
		constant TEXT NOT NULL DEFAULT '',
		UNIQUE (constant)
	);
	`
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
}
