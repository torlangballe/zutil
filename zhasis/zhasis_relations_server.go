//go:build server

package zhasis

import (
	"database/sql"
	"fmt"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zsql"
)

// ClassRelations are the to and from relations of a class and it's parents.
// Overridden relations are not present. Returned by GetRelationsOfClass()
type ClassRelations struct {
	ClassID       int64
	ToRelations   []Relation
	FromRelations []Relation
}

func initRelations() {
	query := `
	CREATE TABLE IF NOT EXISTS relations (
		id SERIAL PRIMARY KEY,
		fromclassid INT NOT NULL,
		verb SMALLINT NOT NULL,
		toclassid INT NOT NULL,
		overrideid INT,
		UNIQUE (fromclassid, verb, toclassid),
		CONSTRAINT fk_fromclassid
		FOREIGN KEY(fromclassid)
		REFERENCES classes(id),
		CONSTRAINT fk_toclassid
		FOREIGN KEY(toclassid)
		REFERENCES classes(id),
		CONSTRAINT fk_overrideid
		FOREIGN KEY(overrideid)
		REFERENCES relations(id)
	);`
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
}

func AddRelationToClass(classID int64, verbName VerbName, toClassID int64, overrideID int64) (relationID int64, err error) {
	verb := verbsToNumbersMap[verbName]
	var o sql.NullInt64
	if overrideID != 0 {
		o.Valid = true
		o.Int64 = overrideID
	}
	query := `
	INSERT INTO relations (fromclassid, verb, toclassid, overrideid) VALUES ($1, $2, $3, $4) ON CONFLICT (fromclassid, verb, toclassid) DO UPDATE SET verb=$2 RETURNING id`
	// row := database.QueryRow(query, classID, userID, verb, toClassID)
	row := database.QueryRow(query, classID, verb, toClassID, o)
	err = row.Scan(&relationID)
	// zlog.Info("RELLINK:", relationID, err)
	if err != nil {
		if err == sql.ErrNoRows {
			return relationID, nil
		}
		zlog.Error(err, zsql.ReplaceDollarArguments(query, classID, verb, toClassID))
		return 0, err
	}
	return relationID, nil
}

func selectRelations(where string) ([]Relation, error) {
	var rels []Relation
	query := "SELECT id, fromclassid, verb, toclassid, overrideid FROM relations WHERE " + where
	rows, err := database.Query(query)
	if err != nil {
		return nil, zlog.Error(err, query)
	}
	for rows.Next() {
		var r Relation
		var overrideID sql.NullInt64
		err = rows.Scan(&r.ID, &r.FromClassID, &r.Verb, &r.ToClassID, &overrideID)
		if err != nil {
			return nil, zlog.Error(err, query)
		}
		if overrideID.Valid {
			r.OverrideID = overrideID.Int64
		}
		rels = append(rels, r)
	}
	return rels, nil
}

func GetRelationsOfClass(classID int64) ([]ClassRelations, error) {
	ids, err := ClassesParentIDs(classID)
	if err != nil {
		zlog.Error(err, "GetRelationsForClass")
		return nil, err
	}
	ids = append([]int64{classID}, ids...)
	csr := make([]ClassRelations, len(ids))
	for _, isTo := range []bool{true, false} {
		err = getRelationsOfClassIDs(ids, isTo, csr)
		if err != nil {
			return nil, zlog.Error(err, "getRelationsOfClassIDs isTo:", isTo)
		}
	}
	return csr, nil
}

func getRelationsOfClassIDs(ids []int64, isTo bool, crs []ClassRelations) error {
	sids := zint.Join64(ids, ",")
	where := "from"
	if isTo {
		where = "to"
	}
	where += "classid IN (" + sids + ")"
	rels, err := selectRelations(where)
	zlog.Info("GetRelationsForClass:", len(rels), err, "Where:", where)
	if err != nil {
		zlog.Error(err, "GetRelationsForClass")
		return nil
	}
	skipRelIDs := map[int64]bool{}
	for i, id := range ids {
		crs[i].ClassID = id
		var skips []int64
		for _, r := range rels {
			if isTo && r.ToClassID != id || !isTo && r.FromClassID != id {
				continue
			}
			if skipRelIDs[r.ID] {
				continue
			}
			if r.OverrideID != 0 {
				skips = append(skips, r.OverrideID)
			}
			if isTo {
				crs[i].ToRelations = append(crs[i].ToRelations, r)
			} else {
				crs[i].FromRelations = append(crs[i].FromRelations, r)
			}
		}
		for _, skid := range skips {
			skipRelIDs[skid] = true
		}
	}
	return nil
}

func FindRelationInSliceForID(relations []Relation, id int64) (*Relation, int) {
	for i, r := range relations {
		if r.ID == id {
			return &relations[i], i
		}
	}
	return nil, -1
}

func relInfo(r Relation) string {
	return fmt.Sprint("R[", numberToVerbNameMap[r.Verb], ":", classInfo(r.FromClassID), "]")

}
