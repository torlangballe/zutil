//go:build server

package zsql

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/lib/pq"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"
)

type SQLCalls zrpc2.CallsBase

type UpdateInfoReceive struct {
	Rows         []zdict.Dict
	TableName    string
	SetColumns   map[string]string
	EqualColumns map[string]string
}

var (
	Calls        = new(SQLCalls)
	MainDatabase *sql.DB
	MainIsSqlite bool
)

func InitMainSQLite(filePath string) error {
	var err error
	MainDatabase, err = NewSQLite(filePath)
	MainIsSqlite = true
	return err
}

func NewSQLite(filePath string) (*sql.DB, error) {
	var err error
	dir, _, sub, _ := zfile.Split(filePath)
	zfile.MakeDirAllIfNotExists(dir)
	file := path.Join(dir, sub+".sqlite")

	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return nil, zlog.Error(err, "open file", file)
	}
	return db, nil
}

// interesting: https://github.com/jmoiron/sqlx
type Executer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

type RowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

type SQLer interface { // this is used for routines that can take a sql.Db or sql.Tx
	QueryRow(query string, args ...interface{}) *sql.Row
	Query(query string, args ...interface{}) (*sql.Rows, error)
	Exec(query string, args ...interface{}) (sql.Result, error)
}

var EscapeReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\t", "\\t",
	"\r", "\\n",
	"'", "''")

var UnescapeReplacer = strings.NewReplacer(
	"\\n", "\n",
	"\\t", "\t",
	"\\r", "\n",
	"''", "'")

var LineOutputReplacer = strings.NewReplacer(
	"\n", "\\n",
	"\t", "\\t",
	"\r", "\\n")

func FieldPointersFromStruct(istruct interface{}, skip []string) (pointers []interface{}) {
	for _, item := range getItems(istruct, skip) {
		a := item.Address
		// zlog.Info("FieldPointersFromStruct:", item.FieldName, item.IsSlice)
		if item.IsSlice {
			_, got := item.Interface.([]byte)
			if !got {
				_, is := item.Interface.(JSONer)
				if !is {
					a = pq.Array(a)
				}
			}
		}
		// zlog.Info("FieldPointersFromStruct:", item.TypeName, item.Kind, item.FieldName, reflect.ValueOf(a).Type())
		pointers = append(pointers, a)
	}
	return
}

func FieldSettingToParametersFromStruct(istruct interface{}, skip []string, prefix string, start int) (set string) {
	for i, item := range getItems(istruct, skip) {
		if i != 0 {
			set += ", "
		}
		set += fmt.Sprintf("%s=$%d", prefix+ConvertFieldName(item), start+i)
	}
	return
}

func FieldParametersFromStruct(istruct interface{}, skip []string, start int) (parameters string) {
	for i := range getItems(istruct, skip) {
		if i != 0 {
			parameters += ", "
		}
		parameters += fmt.Sprintf("$%d", start+i)
	}
	return
}

func GetSQLTypeForReflectKind(item zreflect.Item, isSQLite bool) string {
	var stype string
	switch item.Kind {
	case zreflect.KindBool:
		stype = "SMALLINT"
	case zreflect.KindInt:
		if isSQLite {
			stype = "INTEGER"
			break
		}
		switch item.BitSize {
		case 8, 16:
			stype = "SMALLINT"
		case 32:
			stype = "INT"
		case 64:
			stype = "BIGINT"
		}
	case zreflect.KindFloat:
		stype = "REAL"
	case zreflect.KindString:
		stype = "TEXT"
	case zreflect.KindByte:
		stype = "BLOB"
	case zreflect.KindTime:
		stype = "DATETIME"
	}
	return stype
}

type FieldInfo struct {
	Index       int
	SQLType     string
	IsPrimary   bool
	Kind        zreflect.TypeKind
	FieldName   string
	SQLName     string
	JSONName    string
	SubTagParts []string
}

func FieldValuesFromStruct(istruct interface{}, skip []string) (values []interface{}) {
	for _, item := range getItems(istruct, skip) {
		v := item.Interface
		if item.Kind == zreflect.KindStruct && item.IsPointer {
			v = item.Address
		}
		if item.IsSlice {
			_, got := item.Interface.([]byte)
			if !got {
				v = pq.Array(v)
			}
		}
		values = append(values, v)
	}
	return
}

func FieldInfosFromStruct(istruct interface{}, skip []string, isSQLite bool) (infos []FieldInfo) {
	items, err := zreflect.ItterateStruct(istruct, zreflect.Options{UnnestAnonymous: true})
	if err != nil {
		zlog.Fatal(err, "get items")
	}
	for i, item := range items.Children {
		// zlog.Info("FieldInfosFromStruct:", i, item)
		var name string
		var f FieldInfo
		parts := zreflect.GetTagAsMap(item.Tag)["db"]
		name = ConvertFieldName(item)
		if len(parts) != 0 {
			first := parts[0]
			if first == "-" {
				continue
			}
			f.SubTagParts = parts[1:]
			if first != "" {
				name = first
			}
		}
		if zstr.IndexOf(name, skip) != -1 {
			continue
		}
		f.IsPrimary = (zstr.IndexOf("primary", parts) != -1)
		f.Index = i
		f.SQLName = name
		f.SQLType = GetSQLTypeForReflectKind(item, isSQLite)
		f.Kind = item.Kind
		f.FieldName = item.FieldName
		f.JSONName = zstr.FirstToLower(item.FieldName)
		infos = append(infos, f)
	}
	return infos
}

func CreateSQLite3TableCreateStatementFromStruct(istruct interface{}, table string) (query string, infos []FieldInfo) {
	isSQLite := true
	infos = FieldInfosFromStruct(istruct, nil, isSQLite)
	// zlog.Info("CreateSQLite3TableCreateStatementFromStruct", reflect.ValueOf(istruct).IsZero())
	for _, f := range infos {
		if query == "" {
			query = "CREATE TABLE IF NOT EXISTS " + table + " (\n"
		} else {
			query += ",\n"
		}
		query += `   "` + f.SQLName + `"` + " " + f.SQLType
		if f.IsPrimary {
			query += " NOT NULL PRIMARY KEY AUTOINCREMENT"
		}
	}
	query += "\n);"
	return
}

func (j JSONer) Value() (driver.Value, error) {
	// zlog.Info("JSONer Value:", string(j))
	return driver.Value(j), nil
}

func (j *JSONer) Scan(val interface{}) error {
	if val == nil {
		*j = JSONer{}
		return nil
	}
	// zlog.Info("JSONB Scan", val)
	b, ok := val.([]byte)
	if !ok {
		return zlog.NewError("type assertion to []byte failed", reflect.ValueOf(val).Type())
	}
	*j = JSONer(b)
	return nil
}

var insertQueries = map[string]string{}

func InsertIDStruct(rq RowQuerier, s interface{}, table string) (id int64, err error) {
	key := table + "_insertStruct"
	skip := []string{"id"}
	query := insertQueries[key]
	if query == "" {
		params := FieldParametersFromStruct(s, skip, 1)
		query = "INSERT INTO " + table + " (" + FieldNamesStringFromStruct(s, skip, "") + ") VALUES (" + params + ") RETURNING id"
		insertQueries[key] = query
	}
	vals := FieldValuesFromStruct(s, skip)
	row := rq.QueryRow(query, vals...)
	err = row.Scan(&id)
	zlog.AssertNotError(err, insertQueries, vals)
	return
}

func addTo(i *int, params *[]any, set *[]string, fieldName string, value any, fieldToColumn map[string]string) bool {
	column, got := fieldToColumn[fieldName]
	if !got {
		return false
	}
	s := fmt.Sprint(column, "=$", *i)
	(*i)++
	*set = append(*set, s)
	if reflect.ValueOf(value).Kind() == reflect.Slice {
		value = pq.Array(value)
	}
	*params = append(*params, value)
	return true
}

func (sc *SQLCalls) UpdateStructs(info UpdateInfoReceive) error {
	var params []any
	var sets, wheres []string

	for _, row := range info.Rows {
		i := 1
		for fieldName, value := range row {
			addTo(&i, &params, &sets, fieldName, value, info.SetColumns)
		}
		for fieldName, value := range row {
			addTo(&i, &params, &wheres, fieldName, value, info.EqualColumns)
		}
		query := "UPDATE " + info.TableName + " SET " + strings.Join(sets, ",")
		query += " WHERE " + strings.Join(wheres, ",")
		_, err := MainDatabase.Exec(query, params...)
		zlog.Info("SQLCalls.UpdateStructs:", query, params, err)
		if err != nil {
			return err
		}
	}
	return nil
}

func SelectAnySlice[S any](db *sql.DB, resultSlice *[]S, query string, skipFields []string) error {
	var s S
	rows, err := db.Query(query)
	if err != nil {
		return zlog.Error(err, "select", query)
	}
	defer rows.Close()
	pointers := FieldPointersFromStruct(&s, skipFields)
	for rows.Next() {
		err = rows.Scan(pointers...)
		if err != nil {
			return zlog.Error(err, "select", query)
		}
		*resultSlice = append(*resultSlice, s)
	}
	return nil
}
