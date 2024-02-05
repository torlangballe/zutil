//go:build server

package zsql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"path"
	"reflect"
	"strings"

	"github.com/lib/pq"
	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type Base struct {
	DB   *sql.DB
	Type BaseType
}

type SQLCalls struct{}
type SQLDictSlice []zdict.Dict

var (
	Main                   Base
	GetUserIDFromTokenFunc func(token string) (int64, error)
)

func InitMainSQLite(filePath string) error {
	var err error
	Main.DB, err = NewSQLite(filePath)
	Main.Type = SQLite
	return err
}

func NewSQLite(filePath string) (*sql.DB, error) {
	var err error
	dir, _, sub, _ := zfile.Split(filePath)
	zfile.MakeDirAllIfNotExists(dir)
	file := path.Join(dir, sub+".sqlite")

	db, err := sql.Open("sqlite", file)
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

func (base *Base) CustomizeQuery(query string) string {
	return CustomizeQuery(query, base.Type)
}

func FieldPointersFromStruct(istruct interface{}, skip []string) (pointers []interface{}) {
	ForEachColumn(istruct, skip, "", func(each ColumnInfo) bool {
		// zlog.Info("FieldPointersFromStruct", column, val.CanAddr())
		a := each.ReflectValue.Addr()
		v := a.Interface()
		if a.Kind() == reflect.Slice {
			_, got := a.Interface().([]byte)
			if !got {
				_, is := v.(JSONer)
				if !is {
					v = pq.Array(v)
				}
			}
		}
		pointers = append(pointers, v)
		return true
	})
	return
}

func FieldSettingToParametersFromStruct(istruct interface{}, skip []string, prefix string, start int) string {
	var set string
	var i int
	ForEachColumn(istruct, skip, "", func(each ColumnInfo) bool {
		if i != 0 {
			set += ","
		}
		set += fmt.Sprintf("%s=$%d", prefix+each.Column, start+i)
		i++
		return true
	})
	return set
}

func FieldParametersFromStruct(istruct interface{}, skip []string, start int) (parameters string) {
	var i int
	ForEachColumn(istruct, skip, "", func(ColumnInfo) bool {
		if i != 0 {
			parameters += ","
		}
		parameters += fmt.Sprintf("$%d", start+i)
		i++
		return true
	})
	return
}

func GetSQLTypeForReflectKind(item zreflect.Item, btype BaseType) string {
	var stype string
	switch item.Kind {
	case zreflect.KindBool:
		stype = "SMALLINT"
	case zreflect.KindInt:
		if btype == SQLite {
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
	ForEachColumn(istruct, skip, "", func(each ColumnInfo) bool {
		v := each.ReflectValue.Interface()
		if each.ReflectValue.Kind() == reflect.Ptr {
			v = each.ReflectValue.Addr()
		}
		if each.ReflectValue.Kind() == reflect.Slice {
			_, got := v.([]byte)
			if !got {
				v = pq.Array(v)
			}
		}
		values = append(values, v)
		return true
	})
	return
}

func FieldInfosFromStruct(istruct interface{}, skip []string, btype BaseType) (infos []FieldInfo) {
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
		f.SQLType = GetSQLTypeForReflectKind(item, btype)
		f.Kind = item.Kind
		f.FieldName = item.FieldName
		f.JSONName = zstr.FirstToLower(item.FieldName)
		infos = append(infos, f)
	}
	return infos
}

func CreateSQLite3TableCreateStatementFromStruct(istruct interface{}, table string) (query string, infos []FieldInfo) {
	infos = FieldInfosFromStruct(istruct, nil, SQLite) // hardcode SQLite for now
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

/*
var insertQueries = map[string]string{}

func InsertIDStruct(rq RowQuerier, btype BaseType, ptr interface{}, table string) error {
	rval := reflect.ValueOf(ptr)
	zlog.Assert(rval.Kind() == reflect.Ptr)
	// s := rval.Elem().Interface()
	key := table + "_insertStruct"
	skip := []string{"id"}
	query := insertQueries[key]
	if query == "" {
		params := FieldParametersFromStruct(ptr, skip, 1)
		query = "INSERT INTO " + table + " (" + ColumnNamesStringFromStruct(ptr, skip, "") + ") VALUES (" + params + ") RETURNING id"
		insertQueries[key] = query
	}
	var id int64
	vals := FieldValuesFromStruct(ptr, skip)
	query = CustomizeQuery(query, btype)
	row := rq.QueryRow(query, vals...)
	err := row.Scan(&id)
	if err != nil {
		return err
	}
	finfo, found := zreflect.FieldForName(ptr, zfields.FlattenIfAnonymousOrZUITag, "ID")
	if !found {
		return zlog.NewError("ID not found")
	}
	finfo.ReflectValue.SetInt(id)
	return nil
}

func UpdateIDStruct(ex Executer, btype BaseType, ptr interface{}, table string) error {
	finfo, found := zreflect.FieldForName(ptr, zfields.FlattenIfAnonymousOrZUITag, "ID")
	zlog.Assert(found)
	skip := []string{"id"}
	set := FieldSettingToParametersFromStruct(ptr, skip, "", 1)
	vals := FieldValuesFromStruct(ptr, skip)
	vals = append(vals, finfo.ReflectValue.Interface())
	query := "UPDATE " + table + " SET " + set + fmt.Sprintf(" WHERE id=$%d", len(vals))
	// zlog.Info("Update:", query, len(vals))
	query = CustomizeQuery(query, btype)
	_, err := ex.Exec(query, vals...)
	if err != nil {
		return zlog.Error(err, "exec", query, vals)
	}
	return nil
}

func SelectIDStruct[S any](base *Base, s *S, id int64, table string) error {
	var slice []S
	var q QueryBase
	q.Constraints = fmt.Sprint("WHERE id=", id)
	q.Table = table
	err := SelectSlicesOfAny(base, &slice, q)
	if err != nil {
		return err
	}
	if len(slice) == 0 {
		return zlog.NewError("no slice", id)
	}
	*s = slice[0]
	return nil
}
*/

func getPrimaryColumn(row any) (column string, val any, err error) {
	var got bool
	ForEachColumn(row, nil, "", func(each ColumnInfo) bool {
		// zlog.Info("Each:", each.Column, each.IsPrimary)
		if each.IsPrimary {
			column = each.Column
			val = each.ReflectValue.Interface()
			got = true
			return false
		}
		return true
	})
	if !got {
		err = errors.New("primary column not found")
	}
	return column, val, err
}

func setUserIDInRows[S any](rows []S, userToken string) error {
	userID, err := GetUserIDFromTokenFunc(userToken)
	if err != nil {
		return err
	}
	finfo, found := FieldForColumnName(rows[0], nil, "", "userid")
	if !found {
		return errors.New("No userid column")
	}
	for _, row := range rows {
		finfo := zreflect.FieldForIndex(&row, zfields.FlattenIfAnonymousOrZUITag, finfo.FieldIndex)
		finfo.ReflectValue.SetInt(userID)
	}
	return nil
}

func UpdateRows[S any](table string, rows []S, userToken string) error {
	var idColumn string
	var id any
	if len(rows) == 0 {
		return nil
	}
	idColumn, id, err := getPrimaryColumn(rows[0])
	if err != nil {
		return err
	}
	if userToken != "" {
		err = setUserIDInRows[S](rows, userToken)
		if err != nil {
			return err
		}
	}
	skip := []string{idColumn}
	for _, row := range rows {
		set := FieldSettingToParametersFromStruct(row, skip, "", 1)
		vals := FieldValuesFromStruct(row, skip)
		vals = append(vals, id)
		query := "UPDATE " + table + " SET " + set + " WHERE " + idColumn + fmt.Sprintf("=$%d", len(vals))
		query = CustomizeQuery(query, Main.Type)
		_, err := Main.DB.Exec(query, vals...)
		// zlog.Info("SQLCalls.UpdateRows:", query, vals, err)
		if err != nil {
			return err
		}
	}
	return nil
}

func InsertRows[S any](table string, rows []S, userToken string) (int64, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	idColumn, _, err := getPrimaryColumn(rows[0])
	if err != nil {
		return 0, err
	}
	if userToken != "" {
		err = setUserIDInRows[S](rows, userToken)
		if err != nil {
			return 0, err
		}
	}
	var lastID int64
	idCols := []string{idColumn}
	params := FieldParametersFromStruct(rows[0], idCols, 1)
	query := "INSERT INTO " + table + " (" + ColumnNamesStringFromStruct(rows[0], idCols, "") + ") VALUES (" + params + ") RETURNING " + idColumn
	query = CustomizeQuery(query, Main.Type)
	for _, row := range rows {
		vals := FieldValuesFromStruct(row, idCols)
		dbRow := Main.DB.QueryRow(query, vals...)
		err := dbRow.Scan(&lastID)
		if err != nil {
			return 0, err
		}
	}
	return lastID, nil
}

func (SQLCalls) ExecuteQuery(query string, rowsAffected *int64) error {
	query = CustomizeQuery(query, Main.Type)
	result, err := Main.DB.Exec(query)
	zlog.Info("Exec:", query, err)
	if err != nil {
		return err
	}
	*rowsAffected, _ = result.RowsAffected()
	return nil
}

func SelectSlicesOfAny[S any](base *Base, resultSlice *[]S, q QueryBase) error {
	var s S
	fields := ColumnNamesStringFromStruct(&s, q.SkipColumns, "")
	query := zstr.Spaced("SELECT", fields, "FROM", q.Table)
	if q.Constraints != "" {
		query += " " + q.Constraints
	}
	query = CustomizeQuery(query, base.Type)
	rows, err := base.DB.Query(query)
	if err != nil {
		return zlog.Error(err, "select", query)
	}
	defer rows.Close()
	for rows.Next() {
		var s S
		pointers := FieldPointersFromStruct(&s, q.SkipColumns)
		err = rows.Scan(pointers...)
		if err != nil {
			return zlog.Error(err, "select", query)
		}
		*resultSlice = append(*resultSlice, s)
	}
	return nil
}

func SelectColumnAsSliceOfAny[A any](base *Base, query string, result *[]A) error {
	query = CustomizeQuery(query, base.Type)
	rows, err := base.DB.Query(query)
	if err != nil {
		return zlog.Error(err, "select", query)
	}
	defer rows.Close()
	for rows.Next() {
		var a A
		err = rows.Scan(&a)
		if err != nil {
			return err
		}
		*result = append(*result, a)
	}
	return nil
}

func (SQLCalls) SelectInt64s(query string, result *[]int64) error {
	return SelectColumnAsSliceOfAny(&Main, query, result)
}
