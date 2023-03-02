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
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"
)

type Base struct {
	DB   *sql.DB
	Type BaseType
}

type SQLCalls zrpc2.CallsBase
type SQLDictSlice []zdict.Dict

var (
	Calls = new(SQLCalls)
	Main  Base
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
	ForEachColumn(istruct, skip, "", func(val reflect.Value, column string, primary bool) {
		// zlog.Info("FieldPointersFromStruct", column, val.CanAddr())
		a := val.Addr()
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
	})
	return
}

func FieldSettingToParametersFromStruct(istruct interface{}, skip []string, prefix string, start int) string {
	var set string
	var i int
	ForEachColumn(istruct, skip, "", func(val reflect.Value, column string, primary bool) {
		if i != 0 {
			set += ","
		}
		set += fmt.Sprintf("%s=$%d", prefix+column, start+i)
		i++
	})
	return set
}

func FieldParametersFromStruct(istruct interface{}, skip []string, start int) (parameters string) {
	var i int
	ForEachColumn(istruct, skip, "", func(val reflect.Value, column string, primary bool) {
		if i != 0 {
			parameters += ","
		}
		parameters += fmt.Sprintf("$%d", start+i)
		i++
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
	ForEachColumn(istruct, skip, "", func(val reflect.Value, column string, primary bool) {
		v := val.Interface()
		if val.Kind() == reflect.Ptr {
			v = val.Addr()
		}
		if val.Kind() == reflect.Slice {
			_, got := v.([]byte)
			if !got {
				v = pq.Array(v)
			}
		}
		values = append(values, v)
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
		query = "INSERT INTO " + table + " (" + FieldNamesStringFromStruct(ptr, skip, "") + ") VALUES (" + params + ") RETURNING id"
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
	f, found := zreflect.FindFieldWithNameInStruct("ID", ptr, true)
	if !found {
		return zlog.NewError("ID not found")
	}
	f.SetInt(id)
	return nil
}

func UpdateIDStruct(ex Executer, btype BaseType, ptr interface{}, table string) error {
	rval, got := zreflect.FindFieldWithNameInStruct("ID", ptr, true)
	zlog.Assert(got)
	skip := []string{"id"}
	set := FieldSettingToParametersFromStruct(ptr, skip, "", 1)
	vals := FieldValuesFromStruct(ptr, skip)
	vals = append(vals, rval.Interface())
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

func (sc *SQLCalls) UpdateRows(info UpsertInfo) error {
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
		query = CustomizeQuery(query, Main.Type)
		_, err := Main.DB.Exec(query, params...)
		zlog.Info("SQLCalls.UpdateRows:", query, params, err)
		if err != nil {
			return err
		}
	}
	return nil
}

// InsertRows creates an insert query for all info.Rows
func (sc *SQLCalls) InsertRows(info UpsertInfo, result *UpsertResult) error {
	var params []any
	var columns []string
	fields := zmap.GetKeysAsStrings(info.Rows[0])
	for _, f := range fields {
		columns = append(columns, info.SetColumns[f])
	}
	query := "INSERT INTO " + info.TableName + "(" + strings.Join(columns, ",") + ") VALUES "
	i := 1
	for _, row := range info.Rows {
		var vals string
		for j, f := range fields {
			if j != 0 {
				vals += ","
			}
			vals += fmt.Sprint("$", i)
			i++
			params = append(params, row[f])
		}
		query += "(" + vals + ")"
	}
	hasLastID := (len(info.Rows) == 1 && len(info.EqualColumns) == 1)
	if hasLastID {
		query += " RETURNING " + zmap.GetAnyKeyAsString(info.EqualColumns)
	}
	query = CustomizeQuery(query, Main.Type)
	r, err := Main.DB.Exec(query, params...)
	zlog.Info("SQLCalls.InsertRows:", query, params) //, err)
	if err != nil {
		return err
	}
	if hasLastID {
		result.LastInsertID, _ = r.LastInsertId()
	}
	if info.OffsetQuery != "" {
		row := Main.DB.QueryRow(info.OffsetQuery)
		err := row.Scan(&result.Offset)
		if err != nil {
			return err
		}

	}
	return nil
}

func (sc *SQLCalls) ExecuteQuery(query string, rowsAffected *int64) error {
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
	fields := FieldNamesStringFromStruct(&s, q.SkipFields, "")
	// zlog.Info("SelectSlicesOfAny", fields)
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
	pointers := FieldPointersFromStruct(&s, q.SkipFields)
	for rows.Next() {
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

func (sc *SQLCalls) SelectInt64s(query string, result *[]int64) error {
	return SelectColumnAsSliceOfAny(&Main, query, result)
}
