//go:build server

package zsql

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

func ReplaceDollarArguments(squery string, args ...interface{}) string {
	var to string
	for i, a := range args {
		from := fmt.Sprintf("$%d", i+1)
		switch a.(type) {
		case string, time.Time:
			to = fmt.Sprintf("'%v'", a)
		default:
			to = fmt.Sprintf("%v", a)
		}
		squery = strings.Replace(squery, from, to, -1)
	}
	return squery
}

func SetupPostgres(userName, dbName, address string, ssl bool) (db *sql.DB, err error) {
	sslmode := "disable"
	if ssl {
		sslmode = "require"
	}
	pqStr := fmt.Sprintf(
		"host=%s port=%d sslmode=%s dbname=%s user=%s", //  password=%s
		address,
		5432,
		sslmode,
		dbName,
		userName,
	)

	db, err = sql.Open("postgres", pqStr)
	zlog.Info("OPEN POSTGRES:", pqStr, err)
	if err != nil {
		zlog.Info("setup db err:", err)
		return
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return
}

func PeriodicDump() {
	folder := "dumps/"
	ztimer.Repeat(60, func() bool {
		file := folder + "latest.db"
		if zfile.Exists(file) && time.Since(zfile.Modified(file)) > ztime.Day {
			zfile.MakeDirAllIfNotExists(folder)
			timeFile := time.Now().Format(ztime.ISO8601DateFormat) + ".db"
			os.Rename(file, timeFile)
			zlog.Info("Dump DB")
			zprocess.RunBashCommand("pg_dump etheros > "+file, 0)
		}
		return true
	})
}
