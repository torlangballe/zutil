package zeventdb

import (
	"database/sql"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3" // Import go-sqlite3 library
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
)

type Database struct {
	DB           *sql.DB
	TableName    string
	TimeField    string
	PrimaryField string
	FieldInfos   []zsql.FieldInfo
	StructType   reflect.Type
	istruct      interface{}
}

const TimeStampFormat = "2006-01-02 15:04:05.999999999"

// CreateDB creates (if *filepath* doesn't exist) a file and a sqlite DB pointer opened to it.
// It creates a table *tableName* using the structure istruct as it's column names.
// The field with db:",primary" set, is the primary key.
// If there is more than one time.Time type, set db:",eventtime" tag to make it the event's time.
func CreateDB(filepath string, tableName string, istruct interface{}) (db *Database, err error) {
	if !zfile.Exists(filepath) {
		file, err := os.Create(filepath) // Create SQLite file
		if err != nil {
			return nil, zlog.Error(err, "os.create")
		}
		file.Close()
	}
	db = &Database{}
	db.DB, err = sql.Open("sqlite3", filepath)
	db.TableName = tableName
	db.StructType = reflect.TypeOf(reflect.Indirect(reflect.ValueOf(istruct)).Interface())
	if err != nil {
		return nil, zlog.Error(err, "sql.open")
	}
	var query string
	query, db.FieldInfos = zsql.CreateSQLite3TableCreateStatementFromStruct(istruct, tableName)
	_, err = db.DB.Exec(query)
	if err != nil {
		zlog.Info("\n\n", query, "\n\n")
		zlog.Error(err, "create table", query, tableName)
	}
	for _, f := range db.FieldInfos {
		if f.IsPrimary {
			db.PrimaryField = f.SQLName
		}
		if f.Kind == zreflect.KindTime && (db.TimeField == "" || zstr.IndexOf("eventtime", f.SubTagParts) != -1) {
			db.TimeField = f.SQLName
		}
	}
	return
}

func (db *Database) Add(istruct interface{}) (id int64, err error) {
	skip := []string{db.PrimaryField}
	params := ""
	for i := 0; i < len(db.FieldInfos)-1; i++ {
		if params != "" {
			params += ","
		}
		params += "?"
	}

	query := "INSERT INTO " + db.TableName + " (" + zsql.FieldNamesFromStruct(istruct, skip, "") +
		") VALUES (" + params + ")"
	vals := zsql.FieldValuesFromStruct(istruct, skip)

	zlog.Info("ADD-QUERY:", query, vals)
	r, err := db.DB.Exec(query, vals...)
	if err != nil {
		return 0, zlog.Error(err, "query", query, vals)
	}
	id, err = r.LastInsertId()
	zlog.Info("Add2DB:", id)
	if err != nil {
		return 0, zlog.Error(err, "lastinsert", query, vals)
	}
	return id, nil
}

type CompareItem struct {
	Name   string
	Values []interface{}
}

func (db *Database) Get(resultsSlicePtr interface{}, equalItems zdict.Items, time time.Time, id int64, before bool, count int) error {
	var comps []CompareItem
	for _, e := range equalItems {
		found := false
		for i, c := range comps {
			if c.Name == e.Name {
				comps[i].Values = append(c.Values, e.Value)
				found = true
				break
			}
		}
		if !found {
			c := CompareItem{Name: e.Name, Values: []interface{}{e.Value}}
			comps = append(comps, c)
		}
	}
	var values []interface{}
	var wheres []string
	for _, c := range comps {
		var w string
		if len(c.Values) > 1 {
			w += "("
		}
		for j, v := range c.Values {
			if j != 0 {
				w += " OR "
			}
			values = append(values, v)
			name := c.Name
			if zstr.HasPrefix(name, "!", &name) {
				w += name + "<>?"
			} else {
				w += name + "=?"
			}
		}
		if len(c.Values) > 1 {
			w += ")"
		}
		wheres = append(wheres, w)
	}
	resultStructVal := reflect.New(db.StructType)

	operator := ">"
	if before {
		operator = "<"
	}
	if !time.IsZero() {
		w := db.TimeField + operator + time.Format(TimeStampFormat)
		wheres = append(wheres, w)
	}
	if id != 0 {
		w := db.PrimaryField + operator + strconv.FormatInt(id, 10)
		wheres = append(wheres, w)
	}
	where := strings.Join(wheres, " AND ")
	query := "SELECT " + zsql.FieldNamesFromStruct(resultStructVal.Interface(), nil, "") + " FROM " + db.TableName + " WHERE " + where

	rows, err := db.DB.Query(query, values...)
	if err != nil {
		return zlog.Error(err, "query", query, "vals:", values)
	}
	slicePtrVal := reflect.ValueOf(resultsSlicePtr)
	sliceVal := reflect.Indirect(slicePtrVal)

	resultPointers := zsql.FieldPointersFromStruct(resultStructVal.Interface(), nil)
	for rows.Next() {
		err = rows.Scan(resultPointers...)
		sliceVal = reflect.Append(sliceVal, reflect.Indirect(resultStructVal))
	}
	// zlog.Info("eventsdb.Get:", query, values, wheres, slicePtrVal.CanAddr())
	reflect.Indirect(slicePtrVal).Set(sliceVal)
	return nil
}
