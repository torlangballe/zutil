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
	query := `
CREATE TABLE IF NOT EXISTS classes (
	id SERIAL PRIMARY KEY,
	isid INT REFERENCES classes(id),
	name TEXT NOT NULL DEFAULT '',
	icon TEXT NOT NULL DEFAULT '',
	about TEXT NOT NULL DEFAULT ''
);
`
	_, err := db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
	db.Exec("INSERT INTO classes (id, name, about) VALUES($1, 'zhasisthing', 'root class all others are ancestors of')", RootThingClassID)

	query = `
CREATE TABLE IF NOT EXISTS instances (
	id SERIAL PRIMARY KEY,
	ofid INT,
	userid INT,
	value TEXT NOT NULL DEFAULT '',
	UNIQUE (ofid, userid, value),
	CONSTRAINT fk_ofid
	FOREIGN KEY(ofid)
	REFERENCES classes(id)
);
`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	query = `
CREATE TABLE IF NOT EXISTS links (
	id SERIAL PRIMARY KEY,
	verb SMALLINT NOT NULL,
	toclassid INT NOT NULL,
	UNIQUE (verb, toclassid),
	CONSTRAINT fk_toclassid
	FOREIGN KEY(toclassid)
	REFERENCES classes(id)
);
`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	query = `
CREATE TABLE IF NOT EXISTS relations2 (
	fromclassid INT NOT NULL,
	linkid INT NOT NULL,
	UNIQUE (fromclassid, linkid),
	CONSTRAINT fk_fromclassid
	FOREIGN KEY(fromclassid)
	REFERENCES classes(id),
	CONSTRAINT fk_linkid
	FOREIGN KEY(linkid)
	REFERENCES links(id)
);
`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	FileCache = zfilecache.Init(router, "caches/", "/", "zhasis/")
	FileCache.DeleteAfter = 0

	InitCoreThings()
}

func CreateHardCodedClass(name, about string, hardcodedID, isID int64) (int64, error) {
	var id int64
	query := "INSERT INTO classes (name, about, id, isid) VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, about=EXCLUDED.about, isid=EXCLUDED.isid"
	_, err := database.Exec(query, name, about, hardcodedID, isID)
	if err != nil {
		zlog.Error(err, zsql.ReplaceDollarArguments(query, name, about, hardcodedID, isID))
		return 0, err
	}
	return id, nil
}

func CreateClass(name, about string, isID int64) (int64, error) {
	var id int64
	query := "INSERT INTO classes (name, about, isid) VALUES ($1, $2, $3) RETURNING id"
	row := database.QueryRow(query, name, about, isID)
	err := row.Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func GetClassDescendantClasses(classID int64, recursive bool) ([]Tree, error) {
	var trees []Tree
	query := "SELECT FROM classes id WHERE isid=$1"
	rows, err := database.Query(query, classID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var t Tree
		err = rows.Scan(&t.ID)
		if err != nil {
			return nil, err
		}
		trees = append(trees)
	}
	if !recursive {
		return trees, nil
	}
	for i, t := range trees {
		trees[i].Children, err = GetClassDescendantClasses(t.ID, true)
		if err != nil {
			return nil, err
		}
	}
	return trees, nil
}

func CreateInstance(classID, userID int64, value any) (int64, error) {
	var id int64
	var str string
	if value != "" {
		data, err := json.Marshal(value)
		if err != nil {
			return 0, err
		}
		str = string(data)
	}
	query := "INSERT INTO instances (ofid, userid, value) VALUES ($1, $2, $3) ON CONFLICT (ofid, userid, value) DO UPDATE SET id=EXCLUDED.id RETURNING id"
	row := database.QueryRow(query, classID, userID, str)
	err := row.Scan(&id)
	zlog.Info("Inserted:", id, err)
	if err != nil {
		if err == sql.ErrNoRows {
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, userID, str))
		return 0, err
	}
	return id, nil
}

func AddRelationToClass(classID int64, verbName VerbName, toClassID int64) (linkID int64, err error) {
	verb := verbsToNumbersMap[verbName]
	query := `
	WITH 
	L AS (INSERT INTO links (verb, toclassid) VALUES ($2, $3) ON CONFLICT (verb, toclassid) DO UPDATE SET id=MATCHED.id RETURNING id)
	INSERT INTO relations2 (fromclassid, linkid) SELECT $1, L.id FROM L ON CONFLICT (fromclassid, linkid) DO NOTHING RETURNING linkid`
	// row := database.QueryRow(query, classID, userID, verb, toClassID)
	row := database.QueryRow(query, classID, verb, toClassID)
	err = row.Scan(&linkID)
	zlog.Info("RELLINK:", linkID)
	if err != nil {
		if err == sql.ErrNoRows {
			return linkID, nil
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, verb, toClassID))
		return 0, err
	}
	return linkID, nil
}

func AddValueToInstance(instanceID, userID, linkID, valueInstanceID int64) (err error) {
	// verb := verbsToNumbersMap[verbName]
	// query := `
	// WITH tocid AS (SELECT id FROM classes WHERE id=$4 AND userid=$2 OR userid=0 LIMIT 1),
	// WITH linkid AS (INSERT INTO links (verb, toclassid) VALUES ($3, tocid) RETURNING id"),
	// WITH cid AS (SELECT id FROM classes WHERE id=$1 AND userid=$2 LIMIT 1)
	// INSERT INTO relations (fromclassid, linkid) VALUES (cid, linkid) RETURNING linkid";
	// `
	// row := database.QueryRow(query, classID, userID, verb, toClassID)
	// err = row.Scan(&linkID)
	// if err != nil {
	// 	zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, userID, verb, toClassID))
	// 	return 0, err
	// }
	// return linkID, nil
	return nil
}

func GetRelationsForClass(classID, userID int64) {

}
