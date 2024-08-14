//go:build server

package zhasis

import (
	"database/sql"
	"fmt"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zsql"
)

func initClasses() {
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
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
	database.Exec("INSERT INTO classes (id, name, about) VALUES ($1, 'zhasisthing', 'root class all others are ancestors of')", RootThingClassID)

}

func classString(classID int64) string {
	cs, err := selectClasses(fmt.Sprintf("id=%d", classID))
	if err != nil {
		return "<err>"
	}
	if len(cs) == 0 {
		return fmt.Sprint(classID)
	}
	return fmt.Sprintf("%d*%s", cs[0].ID, cs[0].Name)
}

func selectClasses(where string) ([]Class, error) {
	var classes []Class
	query := "SELECT id, isid, name, icon, about FROM classes"
	query += " WHERE " + where + " AND id<>-1"
	rows, err := database.Query(query)
	if err != nil {
		return nil, zlog.Error(err, query)
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

func ClassesParentIDs(classID int64) ([]int64, error) {
	var ids []int64
	var id sql.NullInt64
	query := `WITH RECURSIVE classHierarchy AS (
    SELECT C1.isid FROM classes C1 WHERE id=$1
  UNION
    SELECT C.isid FROM classes C
	INNER JOIN classHierarchy H
	ON C.id = H.isid
)
SELECT * FROM classHierarchy`
	rows, err := database.Query(query, classID)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		err = rows.Scan(&id)
		if err != nil {
			return nil, zlog.Error(err, "ClassesParentIDs")
		}
		if !id.Valid || id.Int64 == -1 {
			break
		}
		ids = append(ids, id.Int64)
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

func ClassParentAndSelfClasses(classID int64) ([]Class, error) {
	ids, err := ClassesParentIDs(classID)
	if err != nil {
		return nil, err
	}
	ids = append([]int64{classID}, ids...)
	return GetClassesForIDs(ids)
}

func GetClassesForIDs(classIDs []int64) ([]Class, error) {
	where := "id IN (" + zint.Join64(classIDs, ",") + ")"
	classes, err := selectClasses(where)
	return classes, err
}
