//go:build server

package zhasis

import (
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zsql"
)

func initValRels() {
	query := `
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
);`
	_, err := database.Exec(query)
	if err != nil {
		zlog.Fatal(err)
	}
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

func selectValRels(where string) ([]ValRel, error) {
	var vrs []ValRel
	query := "SELECT id, frominstanceid, relationid, valueinstanceid FROM valrels WHERE " + where
	rows, err := database.Query(query)
	if err != nil {
		return nil, zlog.Error(err, query)
	}
	for rows.Next() {
		var vr ValRel
		err = rows.Scan(&vr.ID, &vr.FromInstanceID, &vr.RelationID, &vr.ValueInstanceID)
		if err != nil {
			return nil, err
		}
		vrs = append(vrs, vr)
	}
	return vrs, nil
}
