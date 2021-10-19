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
	Lock         sync.Mutex // sync.RWMutex might cause corruptions, trying regular
}

const TimeStampFormat = "2006-01-02 15:04:05.999999999"

var (
	itemsToStore []interface{}
	storeLock    sync.Mutex
)

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
	zstr.AddToSet(&indexFields, "time")
	for _, field := range indexFields {
		name := strings.Replace(field, ",", "_", -1)
		query = fmt.Sprintf("CREATE INDEX IF NOT EXISTS idx_events_%s ON events (%s)", name, field)
		zlog.Info("index:", query)
		_, err = db.DB.Exec(query)
		zlog.Info("index done:", err)
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
		db.repeatPurge(deleteDays, deleteFreqSecs, tableName)
	}
	go db.repeatWriteItems()
	return
}

func (db *Database) repeatPurge(deleteDays, deleteFreqSecs float64, tableName string) {
	ztimer.RepeatNow(deleteFreqSecs, func() bool {
		start := time.Now()
		zlog.Info("🟩EventDB purged start")
		at := start.Add(-time.Duration(float64(ztime.Day) * deleteDays))
		query := fmt.Sprintf("DELETE FROM %s WHERE time < ?", tableName)
		db.Lock.Lock()
		_, err := db.DB.Exec(query, at)
		db.Lock.Unlock()
		if err != nil {
			zlog.Error(err, "query", query, at)
		}
		zlog.Info("🟩EventDB purged:", time.Since(start))
		return true
	})
}

func (db *Database) Add(istruct interface{}, flush bool) {
	storeLock.Lock()
	itemsToStore = append(itemsToStore, istruct)
	storeLock.Unlock()
	if flush {
		db.writeItems()
	}
}

func (db *Database) repeatWriteItems() {
	for {
		if len(itemsToStore) > 0 {
			db.writeItems()
			time.Sleep(time.Second)
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func (db *Database) writeItems() {
	if len(itemsToStore) == 0 {
		return
	}
	skip := []string{db.PrimaryField}
	params := "(" + strings.Repeat("?,", len(db.FieldInfos)-2) + "?)"

	storeLock.Lock()
	query := "INSERT INTO " + db.TableName + " (" + zsql.FieldNamesFromStruct(itemsToStore[0], skip, "") +
		") VALUES "

	var vals []interface{}
	for i, item := range itemsToStore {
		if i != 0 {
			query += ", "
		}
		query += params
		vals = append(vals, zsql.FieldValuesFromStruct(item, skip)...)
	}
	itemsToStore = itemsToStore[:0]
	storeLock.Unlock()

	//	for c := 0; c < 10; c++ {
	db.Lock.Lock()
	_, err := db.DB.Exec(query, vals...)
	db.Lock.Unlock()
	if err != nil {
		zlog.Error(err, "query", query, vals)
	}
	//	}
}

/*
func (db *Database) repeatWriteItems() {
	for {
		var item interface{}
		zlog.Info("DB.writing?", len(itemsToStore))
		storeLock.Lock()
		if len(itemsToStore) > 0 {
			item = itemsToStore[0]
			itemsToStore = itemsToStore[1:]
		}
		storeLock.Unlock()
		if item == nil {
			time.Sleep(time.Second)
			continue
		}
		db.writeItem(item)
	}
}

func (db *Database) writeItem(istruct interface{}) {
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

	db.Lock.Lock()
	defer db.Lock.Unlock()
	r, err := db.DB.Exec(query, vals...)
	if err != nil {
		zlog.Error(err, "query", query, vals)
		return
	}
	id, err := r.LastInsertId()
	// zlog.Info("Add2DB:", id)
	if err != nil {
		zlog.Error(err, "get lastinsert", query, vals, id)
		return
	}
}
*/

type CompareItem struct {
	Name   string
	Values []interface{}
}

func getTimeCompare(db *Database, t time.Time, start bool) string {
	// zlog.Info("zeventsdb get time compare:", t, start)
	op := ">"
	if !start {
		op = "<"
	}
	w := db.TimeField + op + `'` + t.UTC().Format(TimeStampFormat) + `'`
	return w
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
	if !start.IsZero() {
		wheres = append(wheres, getTimeCompare(db, start, true))
	}
	if !end.IsZero() {
		wheres = append(wheres, getTimeCompare(db, end, false))
	}
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
	db.Lock.Lock()
	zlog.Info("🟩eventsdb.Get:", zsql.ReplaceQuestionMarkArguments(query, values...))
	defer db.Lock.Unlock()
	rows, err := db.DB.Query(query, values...)
	if err != nil {
		return zlog.Error(err, "query", query, "vals:", values)
	}
	defer rows.Close()

	slicePtrVal := reflect.ValueOf(resultsSlicePtr)
	sliceVal := reflect.Indirect(slicePtrVal)

	qtime := time.Since(now)
	resultPointers := zsql.FieldPointersFromStruct(resultStructVal.Interface(), nil)
	for rows.Next() {
		err = rows.Scan(resultPointers...)
		zlog.AssertNotError(err)
		sliceVal = reflect.Append(sliceVal, reflect.Indirect(resultStructVal))
	}
	zlog.Info("🟩eventsdb.Got:", qtime, time.Since(now), sliceVal.Len())
	reflect.Indirect(slicePtrVal).Set(sliceVal)

	return nil
}

func (db *Database) DeleteEvent(id int64) error {
	query := "DELETE FROM events WHERE id=$1"
	db.Lock.Lock()
	_, err := db.DB.Exec(query, id)
	db.Lock.Unlock()
	if err != nil {
		return zlog.Error(err, "delete", query, id)
	}
	return nil
}
