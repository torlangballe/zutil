package zeventdb

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	sqlite "github.com/mattn/go-sqlite3" // Import go-sqlite3 library

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type Database struct {
	DB           *sql.DB
	TableName    string
	TimeField    string
	PrimaryField string
	FieldInfos   []zsql.FieldInfo
	StructType   reflect.Type
	istruct      interface{}
	lock         sync.Mutex // sync.RWMutex might cause corruptions, trying regular
}

const TimeStampFormat = "2006-01-02 15:04:05.999999999"

// CreateDB creates (if *filepath* doesn't exist) a file and a sqlite DB pointer opened to it.
// It creates a table *tableName* using the structure istruct as it's column names.
// The field with db:",primary" set, is the primary key.
// If there is more than one time.Time type, set db:",eventtime" tag to make it the event's time.
func CreateDB(filepath string, tableName string, istruct interface{}, deleteDays, deleteFreqSecs float64, indexFields []string) (db *Database, err error) {
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
	// zlog.Info("ZEDB CREATE:", query)
	_, err = db.DB.Exec(query)
	if err != nil {
		if errors.Is(err, sqlite.ErrCorrupt) || err.Error() == "database disk image is malformed" {
			zlog.Info("CORRUPT!")
			os.Remove(filepath)
			_, err = db.DB.Exec(query)
		}
		if err != nil {
			zlog.Info("\n\n", query, "\n\n")
			zlog.Error(err, "create table", tableName)
			return
		}
	}
	indexFields = append(indexFields, "time")
	for _, field := range indexFields {
		query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_events_%s ON events (%s)", field, field)
		_, err = db.DB.Exec(query)
		if err != nil {
			zlog.Error(err, "create index", query)
		}
	}

	for _, f := range db.FieldInfos {
		if f.IsPrimary {
			db.PrimaryField = f.SQLName
		}
		if f.Kind == zreflect.KindTime && (db.TimeField == "" || zstr.IndexOf("eventtime", f.SubTagParts) != -1) {
			db.TimeField = f.SQLName
		}
	}
	if deleteDays != 0 && deleteFreqSecs != 0 {
		ztimer.RepeatNow(deleteFreqSecs, func() bool {
			start := time.Now()
			zlog.Info("🟩EventDB purged start")
			at := start.Add(-time.Duration(float64(ztime.Day) * deleteDays))
			query := fmt.Sprintf("DELETE FROM %s WHERE time < ?", tableName)
			db.lock.Lock()
			_, err := db.DB.Exec(query, at)
			db.lock.Unlock()
			if err != nil {
				zlog.Error(err, "query", query, at)
			}
			zlog.Info("🟩EventDB purged:", time.Since(start))
			return true
		})
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

	// zlog.Info("ADD-QUERY:", query, vals)

	db.lock.Lock()
	defer db.lock.Unlock()
	r, err := db.DB.Exec(query, vals...)
	if err != nil {
		return 0, zlog.Error(err, "query", query, vals)
	}
	id, err = r.LastInsertId()
	// zlog.Info("Add2DB:", id)
	if err != nil {
		return 0, zlog.Error(err, "get lastinsert", query, vals)
	}
	return id, nil
}

type CompareItem struct {
	Name   string
	Values []interface{}
}

func getTimeCompare(db *Database, addTo *[]string, t time.Time, start bool) {
	// zlog.Info("zeventsdb get time compare:", t, start)
	if t.IsZero() {
		return
	}
	op := ">"
	if !start {
		op = "<"
	}
	w := db.TimeField + op + `'` + t.UTC().Format(TimeStampFormat) + `'`
	*addTo = append(*addTo, w)
}

func (db *Database) Get(resultsSlicePtr interface{}, equalItems zdict.Items, start, end time.Time, startID, endID int64, decending bool, keepID int64, count int) error {
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
			name := c.Name
			sval, _ := v.(string)
			if sval != "" && strings.Contains(sval, "*") {
				sval = strings.Replace(sval, "*", "%", -1)
				w += name + " LIKE ?"
				values = append(values, sval)
				continue
			}
			values = append(values, v)
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

	dir := "ASC"
	if decending {
		dir = "DESC"
	}
	getTimeCompare(db, &wheres, start, true)
	getTimeCompare(db, &wheres, end, false)
	if startID != 0 || endID != 0 {
		op := ">"
		id := startID
		if endID != 0 {
			op = "<"
			id = endID
		}
		w := db.PrimaryField + op + strconv.FormatInt(id, 10)
		wheres = append(wheres, w)
	}
	where := strings.Join(wheres, " AND ")
	query := "SELECT " + zsql.FieldNamesFromStruct(resultStructVal.Interface(), nil, "") + " FROM " + db.TableName
	if keepID != 0 {
		where = "(" + where + fmt.Sprint(") OR id=", keepID)
	}
	query += " WHERE " + where
	query += " ORDER BY " + db.TimeField + " " + dir
	if count != 0 {
		query += fmt.Sprint(" LIMIT ", count)
	}
	now := time.Now()
	db.lock.Lock()
	aquery := zsql.ReplaceQuestionMarkArguments(query, values...)
	zlog.Info("🟩eventsdb.Get:", aquery)
	defer db.lock.Unlock()
	rows, err := db.DB.Query(query, values...)
	if err != nil {
		return zlog.Error(err, "query", query, "vals:", values)
	}
	defer rows.Close()

	slicePtrVal := reflect.ValueOf(resultsSlicePtr)
	sliceVal := reflect.Indirect(slicePtrVal)

	resultPointers := zsql.FieldPointersFromStruct(resultStructVal.Interface(), nil)
	for rows.Next() {
		err = rows.Scan(resultPointers...)
		zlog.AssertNotError(err)
		sliceVal = reflect.Append(sliceVal, reflect.Indirect(resultStructVal))
	}
	zlog.Info("🟩eventsdb.Got:", time.Since(now), sliceVal.Len())
	reflect.Indirect(slicePtrVal).Set(sliceVal)

	return nil
}

func (db *Database) DeleteEvent(id int64) error {
	query := "DELETE FROM events WHERE id=$1"
	db.lock.Lock()
	_, err := db.DB.Exec(query, id)
	db.lock.Unlock()
	if err != nil {
		return zlog.Error(err, "delete", query, id)
	}
	return nil
}
