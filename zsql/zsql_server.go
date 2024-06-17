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
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type Base struct {
	DB             *sql.DB
	Type           BaseType
	statementCache map[string]*sql.Stmt
}

type SQLCalls struct{}

// type SQLDictSlice []zdict.Dict

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
	dir, _, sub, ext := zfile.Split(filePath)
	zfile.MakeDirAllIfNotExists(dir)
	if ext != ".sqlite" {
		ext = ".sqlite3"
	}
	file := path.Join(dir, sub+ext)

	db, err := sql.Open("sqlite", file)
	if err != nil {
		return nil, zlog.Error(err, "open file", file)
	}
	return db, nil
}

// interesting: https://github.com/jmoiron/sqlx
type Executer interface {
	Exec(query string, args ...any) (sql.Result, error)
}

type RowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

type SQLer interface { // this is used for routines that can take a sql.Db or sql.Tx
	QueryRow(query string, args ...any) *sql.Row
	Query(query string, args ...any) (*sql.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
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

// func makeAnyWithPQArrayIfSlice(rval reflect.Value, kind reflect.Kind) any {
// 	a := rval.Interface()
// 	if kind == reflect.Slice {
// 		v := rval.Interface()
// 		_, got := v.([]byte)
// 		if !got {
// 			_, is := v.(JSONer)
// 			if !is {
// 				_, is := v.(sql.Scanner)
// 				if !is {
// 					return pq.Array(a)
// 				}
// 			}
// 		}
// 	}
// 	return a
// }

func FieldPointersFromStruct(istruct any, skip []string) (pointers []any) {
	ForEachColumn(istruct, skip, "", func(each ColumnInfo) bool {
		a := each.ReflectValue.Addr().Interface()
		_, isScanner := a.(sql.Scanner)
		if !isScanner && each.ReflectValue.Kind() == reflect.Slice {
			a = pq.Array(a)
		}
		pointers = append(pointers, a)
		return true
	})
	return
}

func FieldValuesFromStruct(istruct any, skip []string) (values []any) {
	ForEachColumn(istruct, skip, "", func(each ColumnInfo) bool {
		a := each.ReflectValue.Interface()
		_, isValuer := a.(driver.Valuer)
		// zlog.Info("FieldValuesFromStruct:", isValuer, each.ReflectValue.Kind(), each.ReflectValue.Type(), each.Column)
		if !isValuer && each.ReflectValue.Kind() == reflect.Slice {
			a = pq.Array(a)
		}
		values = append(values, a)
		return true
	})
	return
}

func FieldSettingToParametersFromStruct(istruct any, skip []string, prefix string, start int) string {
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

func FieldParametersFromStruct(istruct any, skip []string, start int) (parameters string) {
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

func FieldInfosFromStruct(istruct any, skip []string, btype BaseType) (infos []FieldInfo) {
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

func CreateSQLite3TableCreateStatementFromStruct(istruct any, table string) (query string, infos []FieldInfo) {
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

func (j *JSONer) Scan(val any) error {
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

func getSpecialColumns(row any) (primaryCol, uidCol string, pval, uval any, err error) {
	var got int
	ForEachColumn(row, nil, "", func(each ColumnInfo) bool {
		// zlog.Info("Each:", each.Column, each.IsPrimary)
		if each.IsPrimary {
			primaryCol = each.Column
			pval = each.ReflectValue.Interface()
			got++
			return got != 2
		}
		if each.IsUserID {
			uidCol = each.Column
			uval = each.ReflectValue.Interface()
			got++
			return got != 2
		}
		return true
	})
	return primaryCol, uidCol, pval, uval, err
}

func setUserIDInRows[S any](rows []S, uidColumn, token string) (int64, error) {
	userID, err := GetUserIDFromTokenFunc(token)
	if err != nil {
		return 0, err
	}
	finfo, found := FieldForColumnName(rows[0], nil, "", uidColumn)
	if !found {
		return 0, errors.New("No userid column or UserIDSetter")
	}
	for i := range rows {
		finfo := zreflect.FieldForIndex(&rows[i], nil, finfo.FieldIndex)
		finfo.ReflectValue.SetInt(userID)
	}
	return userID, nil
}

func UpdateRows[S any](table string, rows []S, userToken string) error {
	if len(rows) == 0 {
		return nil
	}
	idCol, userIDCol, id, userID, err := getSpecialColumns(rows[0])
	if err != nil {
		return err
	}

	skip := []string{idCol}
	if userIDCol != "" {
		skip = append(skip, userIDCol)
	}
	if userID == 0 && userToken != "" {
		userID, err = GetUserIDFromTokenFunc(userToken)
		if err != nil {
			return err
		}
	}
	// zlog.Info("zsql.UpdateRows:", table, len(rows))
	for _, row := range rows {
		set := FieldSettingToParametersFromStruct(row, skip, "", 1)
		vals := FieldValuesFromStruct(row, skip)
		vals = append(vals, id)
		query := "UPDATE " + table + " SET " + set + " WHERE " + idCol + fmt.Sprintf("=$%d", len(vals))
		if userIDCol != "" {
			query += fmt.Sprintf(" AND %s=%v", userIDCol, userID)
		}
		query = CustomizeQuery(query, Main.Type)
		_, err := Main.DB.Exec(query, vals...)
		// zlog.Info("SQLCalls.UpdateRows2:", query, vals, err)
		if err != nil {
			return err
		}
	}
	return nil
}

func InsertRows[S any](table string, rows []S, skipColumns []string, userToken string) ([]int64, error) {
	// zlog.Info("InsertRows1:", table, len(rows))
	var ids []int64
	if len(rows) == 0 {
		return nil, nil
	}
	idCol, uidCol, _, _, err := getSpecialColumns(rows[0])
	if err != nil {
		return nil, err
	}
	if userToken != "" && uidCol != "" {
		_, err = setUserIDInRows[S](rows, uidCol, userToken)
		if err != nil {
			return nil, err
		}
	}
	var lastID int64
	skip := append(skipColumns, idCol)
	params := FieldParametersFromStruct(rows[0], skip, 1)
	query := "INSERT INTO " + table + " (" + ColumnNamesStringFromStruct(rows[0], skip, "") + ") VALUES (" + params + ") RETURNING " + idCol
	query = CustomizeQuery(query, Main.Type)
	// zlog.Info("InsertRows:", table, len(rows), query)
	for _, row := range rows {
		vals := FieldValuesFromStruct(row, skip)
		dbRow := Main.DB.QueryRow(query, vals...)
		err := dbRow.Scan(&lastID)
		if err != nil {
			return ids, zlog.Error(err, query, vals)
		}
		ids = append(ids, lastID)
	}
	return ids, nil
}

func UpsertRow[S any](table, conflictCol string, row S, skipColumns []string, idCol, userToken, where string) (id int64, native string, err error) {
	var sets []string
	params := FieldParametersFromStruct(row, skipColumns, 1)
	vals := FieldValuesFromStruct(row, skipColumns)
	for _, n := range ColumnNamesFromStruct(row, skipColumns, "") {
		sets = append(sets, n+"=EXCLUDED."+n)
	}
	query := "INSERT INTO " + table + " (" + ColumnNamesStringFromStruct(row, skipColumns, "") + ") VALUES (" + params + ")\n"
	query += "ON CONFLICT (" + conflictCol + ")\n"
	query += "DO UPDATE SET " + strings.Join(sets, ",") + "\n"
	if where != "" {
		query += "WHERE " + where + "\n"
	}
	query += " RETURNING " + idCol + "," + conflictCol + "\n"

	dbRow := Main.DB.QueryRow(query, vals...)
	err = dbRow.Scan(&id, &native)
	if err == sql.ErrNoRows {
		err = nil
	}
	if zlog.OnError(err, ReplaceDollarArguments(query, vals...)) {
		return 0, "", err
	}
	return id, native, nil
}

func UpsertRows[S any](table, conflictCol string, rows []S, skipColumns []string, userToken string) (map[string]int64, error) {
	if len(rows) == 0 {
		return nil, nil
	}
	var where string
	idCol, uidCol, _, _, err := getSpecialColumns(rows[0])
	if err != nil {
		return nil, err
	}
	if userToken != "" && uidCol != "" {
		userID, err := setUserIDInRows[S](rows, uidCol, userToken)
		if err != nil {
			return nil, err
		}
		where = fmt.Sprintf("%s.%s=%d", table, uidCol, userID)
	}
	zstr.AddToSet(&skipColumns, idCol)

	m := map[string]int64{}
	for _, row := range rows {
		id, native, err := UpsertRow(table, conflictCol, row, skipColumns, idCol, userToken, where)
		if err != nil {
			return nil, err
		}
		m[native] = id
	}
	return m, nil
}

func (SQLCalls) ExecuteQuery(query string, rowsAffected *int64) error {
	query = CustomizeQuery(query, Main.Type)
	result, err := Main.DB.Exec(query)
	// zlog.Info("Exec:", query, err)
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

	// zlog.Info("SelectSlicesOfAny:", query)
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

func (b *Base) Exec(query string, args ...any) (sql.Result, error) {
	var err error
	if b.statementCache == nil {
		b.statementCache = map[string]*sql.Stmt{}
	}
	s := b.statementCache[query]
	if s == nil {
		s, err = b.DB.Prepare(query)
		if err != nil {
			return nil, err
		}
		b.statementCache[query] = s
	}
	return s.Exec(args...)
}
