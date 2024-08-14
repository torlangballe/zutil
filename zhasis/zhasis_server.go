//go:build server

package zhasis

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zsql"
)

type HasIsCalls zrpc.CallsBase

var (
	database *sql.DB

	FileCache            *zfilecache.Cache
	PublicOnlyAdminError = errors.New("only administrators can add public relations")
)

func Init(db *sql.DB, router *mux.Router) {
	database = db
	initClasses()
	initInstances()
	initConstants()
	initRelations()
	initValRels()

	FileCache = zfilecache.Init(router, "caches/", "/", "zhasis/")
	FileCache.DeleteAfter = 0

	InitCoreThings()
}

func CreateConstant(constant any) (int64, error) {
	var id int64
	var str string
	if constant != nil {
		data, err := json.Marshal(constant)
		if err != nil {
			return 0, err
		}
		str = string(data)
	}
	query := "INSERT INTO constants (constant) VALUES ($1) ON CONFLICT (constant) DO UPDATE SET constant=EXCLUDED.constant RETURNING id"
	row := database.QueryRow(query, str)
	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, constant))
		return 0, err
	}
	return id, nil
}

// func AddConstantToInstance(instanceID, relationID, constant any, userID int64) error {
// 	instID, err := CreateInstanceWithConstant(classID, userID int64, constant any) (int64, error) {
// }
// }
