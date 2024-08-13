//go:build server

package zhasis

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zfilecache"
	"github.com/torlangballe/zutil/zint"
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
	about TEXT NOT NULL DEFAULT '',
	UNIQUE (id, name)
);
`
	_, err := db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
	db.Exec("INSERT INTO classes (id, name, about) VALUES ($1, 'zhasisthing', 'root class all others are ancestors of')", RootThingClassID)

	query = `
CREATE TABLE IF NOT EXISTS instances (
	id SERIAL PRIMARY KEY,
	ofid INT,
	userid INT,
	constantid INT NOT NULL,
	UNIQUE (ofid, userid, constantid),
	CONSTRAINT fk_ofid
	FOREIGN KEY(ofid)
	REFERENCES classes(id),
	CONSTRAINT fk_constantid
	FOREIGN KEY(constantid)
	REFERENCES constants(id)
);
`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	query = `
	CREATE TABLE IF NOT EXISTS constants (
		id SERIAL PRIMARY KEY,
		constant TEXT NOT NULL DEFAULT '',
		UNIQUE (constant)
	);
	`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	query = `
CREATE TABLE IF NOT EXISTS relations (
	id SERIAL PRIMARY KEY,
	fromclassid INT NOT NULL,
	verb SMALLINT NOT NULL,
	toclassid INT NOT NULL,
	overrideid INT,
	UNIQUE (fromclassid, verb, toclassid, overrideid),
	CONSTRAINT fk_fromclassid
	FOREIGN KEY(fromclassid)
	REFERENCES classes(id),
	CONSTRAINT fk_toclassid
	FOREIGN KEY(toclassid)
	REFERENCES classes(id),
	CONSTRAINT fk_overrideid
	FOREIGN KEY(overrideid)
	REFERENCES relations(id)
);
`
	_, err = db.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}

	query = `
CREATE TABLE IF NOT EXISTS valrels (
	id SERIAL PRIMARY KEY,
	frominstanceid INT NOT NULL,
	relationid INT NOT NULL,
	valueinstanceid INT NOT NULL,
	UNIQUE (frominstanceid, relationid, valueinstanceid),
	CONSTRAINT kf_frominstanceid
	FOREIGN KEY(frominstanceid)
	REFERENCES instances(id),
	CONSTRAINT fk_relationid
	FOREIGN KEY(relationid)
	REFERENCES relations(id),
	CONSTRAINT kf_valueinstanceid
	FOREIGN KEY(valueinstanceid)
	REFERENCES instances(id)
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

func ClassesParentIDs(classID int64) ([]int64, error) {
	var ids []int64
	var id int64
	query := `WITH RECURSIVE classHierary AS (
    SELECT C1.isid FROM classes C1 WHERE id=$1
  UNION
    SELECT C.isid FROM classes C
	INNER JOIN classHierary H
	ON C.id = H.isid
)
SELECT * FROM classHierary`
	rows, err := database.Query(query, classID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
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

func GetClassParentClasses(classID int64) ([]Class, error) {
	ids, err := ClassesParentIDs(classID)
	if err != nil {
		return nil, err
	}
	return GetClassesForIDs(ids)
}

func GetClassesForIDs(classIDs []int64) ([]Class, error) {
	var classes []Class
	format := "SELECT id, isid, name, about, icon FROM classes id WHERE id IN (%s)"
	query := fmt.Sprintf(format, zint.Join64(classIDs, ","))
	rows, err := database.Query(query)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c Class
		err = rows.Scan(&c.ID, &c.IsID, &c.Name, &c.About, &c.Icon)
		if err != nil {
			return nil, err
		}
		classes = append(classes, c)
	}
	return classes, nil
}

func CreateInstance(classID, userID, constantID int64) (int64, error) {
	var id int64
	query := "INSERT INTO instances (ofid, userid, constantid) VALUES ($1, $2, $3) ON CONFLICT (ofid, userid, constantid) DO UPDATE SET userid=EXCLUDED.userid RETURNING id"
	row := database.QueryRow(query, classID, userID, constantID)
	err := row.Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, userID, constantID))
		return 0, err
	}
	return id, nil
}

func CreateInstanceWithConstant(classID, userID int64, constant any) (int64, error) {
	constID, err := CreateConstant(constant)
	if err != nil {
		return 0, err
	}
	return CreateInstance(classID, userID, constID)
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

func AddRelationToClass(classID int64, verbName VerbName, toClassID int64, overrideID *int64) (relationID int64, err error) {
	verb := verbsToNumbersMap[verbName]
	var o sql.NullInt64
	if overrideID != nil {
		o.Valid = true
		o.Int64 = *overrideID
	}
	query := `
	INSERT INTO relations (fromclassid, verb, toclassid, overrideid) VALUES ($1, $2, $3, $4) ON CONFLICT (fromclassid, verb, toclassid, overrideid) DO UPDATE SET verb=$2 RETURNING id`
	// row := database.QueryRow(query, classID, userID, verb, toClassID)
	row := database.QueryRow(query, classID, verb, toClassID, o)
	err = row.Scan(&relationID)
	zlog.Info("RELLINK:", relationID, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return relationID, nil
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, verb, toClassID))
		return 0, err
	}
	return relationID, nil
}

func AddValueRelationToInstance(instanceID, relationID, valueInstanceID, userID int64) error {
	query := `
	WITH F AS (SELECT id FROM instances WHERE id=$1 AND ($4=0 OR userid=$4)),
	     V AS (SELECT id FROM instances WHERE id=$3 AND ($4=0 OR userid=$4))
	INSERT INTO valrels (frominstanceid, relationid, valueinstanceid) SELECT F.id, $2, V.id FROM F,V LIMIT 1 ON CONFLICT (frominstanceid, relationid, valueinstanceid) DO UPDATE SET relationid=$2;
	`
	_, err := database.Exec(query, instanceID, relationID, valueInstanceID, userID)
	if err != nil {
		zlog.Error(err, zsql.ReplaceDollarArguments(query, instanceID, relationID, valueInstanceID, userID))
		return err
	}
	return nil
}

// func AddConstantToInstance(instanceID, relationID, constant any, userID int64) error {
// 	instID, err := CreateInstanceWithConstant(classID, userID int64, constant any) (int64, error) {
// }
// }

func GetRelationsForClass(classID, userID int64) {

}
