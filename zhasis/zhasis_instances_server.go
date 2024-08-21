//go:build server

package zhasis

import (
	"database/sql"
	"fmt"
	"strconv"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zsql"
)

func initInstances() {
	query := `
CREATE TABLE IF NOT EXISTS instances (
	id SERIAL PRIMARY KEY,
	ofclassid INT NOT NULL,
	userid INT NOT NULL DEFAULT 0,
	constantid INT NOT NULL,
	UNIQUE (ofclassid, userid, constantid),
	CONSTRAINT fk_ofclassid
	FOREIGN KEY(ofclassid)
	REFERENCES classes(id),
	CONSTRAINT fk_constantid
	FOREIGN KEY(constantid)
	REFERENCES constants(id)
);`
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
}

func instanceInfo(instanceID int64) string {
	str := strconv.FormatInt(instanceID, 10) + "*"
	inst, err := getInstance(instanceID)
	if err != nil {
		str += "<err>"
	} else {
		str += classInfo(inst.OfClassID)
	}
	str += ":" + constantInfo(inst.ConstantID)
	return str
}

func CreateInstance(classID, userID, constantID int64) (int64, error) {
	var id int64
	query := "INSERT INTO instances (ofclassid, userid, constantid) VALUES ($1, $2, $3) ON CONFLICT (ofclassid, userid, constantid) DO UPDATE SET userid=EXCLUDED.userid RETURNING id"
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

func getInstance(id int64) (Instance, error) {
	where := fmt.Sprintf("id=%d", id)
	is, err := selectInstances(where)
	if err != nil {
		return Instance{}, err
	}
	if len(is) == 0 {
		return Instance{}, NotFoundError
	}
	return is[0], nil
}

func selectInstances(where string) ([]Instance, error) {
	var is []Instance
	query := "SELECT id, ofclassid, userid, constantid FROM instances WHERE " + where
	rows, err := database.Query(query)
	if err != nil {
		return nil, zlog.Error(err, query)
	}
	for rows.Next() {
		var inst Instance
		err = rows.Scan(&inst.ID, &inst.OfClassID, &inst.UserID, &inst.ConstantID)
		if err != nil {
			return nil, zlog.Error(err, query)
		}
		is = append(is, inst)
	}
	return is, nil
}

func classIDOfInstanceID(instanceID int64) (int64, error) {
	var id int64
	row := database.QueryRow("SELECT ofclassid FROM instances WHERE id=$1", instanceID)
	err := row.Scan(&id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

type InstanceValue struct {
	ValueInstanceID int64
	Relation        Relation
}

func GetValuesOfInstance(instanceID int64) (froms, values []InstanceValue, err error) {
	classID, err := classIDOfInstanceID(instanceID)
	if err != nil {
		return nil, nil, zlog.Error(err, instanceID)
	}
	crs, err := GetRelationsOfClass(classID)
	if err != nil {
		return nil, nil, zlog.Error(err, classID)
	}
	var fromRels, toRels []Relation
	for _, cr := range crs {
		for _, rf := range cr.FromRelations {
			fromRels = append(fromRels, rf)
		}
		for _, rt := range cr.ToRelations {
			toRels = append(toRels, rt)
		}
	}
	froms, err = getValueRelations(fromRels, false, instanceID)
	if err != nil {
		return nil, nil, err
	}
	values, err = getValueRelations(toRels, true, instanceID)
	if err != nil {
		return nil, nil, err
	}
	return froms, values, nil
}

func getValueRelations(rels []Relation, isTo bool, instanceID int64) ([]InstanceValue, error) {
	var ids []int64
	var ivs []InstanceValue
	column := "from"
	if isTo {
		column = "value"
	}
	column += "instanceid"
	for _, r := range rels {
		ids = append(ids, r.ID)
	}
	sids := zint.Join64(ids, ",")
	where := fmt.Sprintf("%s=%d AND relationid IN (%s)", column, instanceID, sids)
	vrs, err := selectValRels(where)
	if err != nil {
		return nil, err
	}
	for _, vr := range vrs {
		var iv InstanceValue
		if isTo {
			iv.ValueInstanceID = vr.FromInstanceID
		} else {
			iv.ValueInstanceID = vr.ValueInstanceID
		}
		r, _ := FindRelationInSliceForID(rels, vr.RelationID)
		zlog.Assert(r != nil)
		iv.Relation = *r
		ivs = append(ivs, iv)
	}
	return ivs, err
}
