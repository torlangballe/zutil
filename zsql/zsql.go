package zsql

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type (
	JSONer             []byte // TODO: Remove? Use sql.Scan/Value methods instead?
	JSONStringArrayPtr []string
	JSONStringArray    []string
)

type UpsertInfo[S any] struct {
	Rows [][]S
	// Insert zbool.BoolInd
}

// InsertInfo can be simlified? Not used as upsert, oly insert
type InsertInfo[S any] struct {
	UpsertInfo[S]
	DefCols []string
}

// type UpsertResult struct {
// 	LastInsertID int64
// 	Offset       int64
// }

type QueryBase struct {
	Constraints string
	Table       string
	SkipColumns []string
}

type BaseType int

// UserIDer is an interface a struct can get a field that represents a user id with.
type UserIDer interface {
	GetUserID() int64
}

const (
	NoType          = 0
	SQLite BaseType = iota + 1
	Postgres
	MySQL // Not used yet
)

var (
	timeType            = reflect.TypeOf(time.Time{})
	AllreadyExistsError = errors.New("already exists")
)

// CustomizeQuery can make a sqlite or psql (or other in future) query, replacing $x with ?x for sqlite.
// It also replaces $NOW, $SERIAL and $PRIMARY-INT-INC with what is needed for each DB type.
func CustomizeQuery(query string, btype BaseType) string {
	switch btype {
	case SQLite:
		query = strings.Replace(query, "$SERIAL", "BIGINT", -1)
		query = strings.Replace(query, "$NOW", "CURRENT_TIMESTAMP", -1)
		query = strings.Replace(query, "$PRIMARY-INT-INC", "INTEGER PRIMARY KEY AUTOINCREMENT", -1)
		query = strings.Replace(query, "$", "?", -1)
	case Postgres:
		query = strings.Replace(query, "$SERIAL", "SERIAL", -1)
		query = strings.Replace(query, "$NOW", "NOW()", -1)
		query = strings.Replace(query, "$PRIMARY-INT-INC", "SERIAL PRIMARY KEY", -1)

	default:
		panic("bad base")
	}
	return query
}

func ReplaceQuestionMarkArguments(squery string, args ...interface{}) string {
	for _, a := range args {
		var sa string
		t, got := a.(time.Time)
		if got {
			sa = `'` + t.Format("2006-01-02 15:04:05.999999999Z07:00"+`'`)
		} else {
			str, got := a.(string)
			if got {
				sa = "'" + str + "'"
			} else {
				sa = fmt.Sprint(a)
			}
		}
		squery = strings.Replace(squery, "?", sa, 1)
	}
	return squery
}

func QuoteString(str string) string {
	return "'" + SanitizeString(str) + "'"
}

func SanitizeString(str string) string {
	return strings.Replace(str, "'", "''", -1)
}

// func ConvertFieldName(i zreflect.Item) string {
// 	return strings.ToLower(i.FieldName)
// }

func ColumnNamesStringFromStruct(istruct interface{}, skip []string, prefix string) string {
	fs := ColumnNamesFromStruct(istruct, skip, prefix)
	return strings.Join(fs, ",")
}

func ColumnNamesFromStruct(s interface{}, skip []string, prefix string) []string {
	var fields []string
	ForEachColumn(s, skip, prefix, func(each ColumnInfo) bool {
		fields = append(fields, each.Column)
		return true
	})
	return fields
}

func FieldNamesToColumnFromStruct(s interface{}, skip []string, prefix string) (m map[string]string, primaryCol string) {
	m = map[string]string{}
	ForEachColumn(s, skip, prefix, func(each ColumnInfo) bool {
		if each.IsPrimary {
			primaryCol = each.Column
		}
		m[each.StructField.Name] = each.Column
		return true
	})
	return m, primaryCol
}

func FieldForColumnName(s interface{}, skip []string, prefix, column string) (ColumnInfo, bool) {
	var finfo ColumnInfo
	var found bool
	ForEachColumn(s, skip, prefix, func(each ColumnInfo) bool {
		if each.Column == column {
			finfo = each
			found = true
			return false
		}
		return true
	})
	return finfo, found
}

func FieldIndexForColumnName(s interface{}, skip []string, prefix, column string) int {
	var index = -1
	ForEachColumn(s, skip, prefix, func(each ColumnInfo) bool {
		if each.Column == column {
			index = each.FieldInfo.FieldIndex
			return false
		}
		return true
	})
	return index
}

type ColumnInfo struct {
	zreflect.FieldInfo
	Column      string
	IsPrimary   bool
	IsUserID    bool
	ColumnIndex int
}

func ForEachColumn(s interface{}, skip []string, prefix string, got func(each ColumnInfo) bool) {
	i := 0
	zreflect.ForEachField(s, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		var colInfo ColumnInfo
		colInfo.FieldInfo = each
		dbTags := zreflect.GetTagAsMap(string(each.StructField.Tag))["db"]
		if len(dbTags) == 0 || dbTags[0] == "" {
			colInfo.Column = strings.ToLower(each.StructField.Name)
		} else {
			colInfo.Column = dbTags[0]
			if colInfo.Column == "-" {
				return true
			}
		}
		if zstr.StringsContain(skip, colInfo.Column) {
			return true
		}
		if each.ReflectValue.Kind() == reflect.Struct && each.ReflectValue.Type() != timeType {
			valuer, _ := each.ReflectValue.Interface().(driver.Valuer)
			if valuer == nil {
				zlog.Info("HERE:", each.StructField.Name)
				var a any
				if each.ReflectValue.CanAddr() {
					a = each.ReflectValue.Addr().Interface()
				} else {
					a = each.ReflectValue.Interface()
				}
				ForEachColumn(a, skip, prefix+colInfo.Column, got)
				return true
			}
		}
		colInfo.ColumnIndex = i
		colInfo.IsPrimary = zstr.IndexOf("primary", dbTags) > 0
		colInfo.IsUserID = zstr.IndexOf("userid", dbTags) > 0
		i++
		return got(colInfo)
	})
}

func Value(a any) (d driver.Value, err error) {
	// return json.Marshal(a)
	v, err := json.Marshal(a)
	// zlog.Info("SQL Value", string(v), err)
	return v, err
}

func Scan[A any](a *A, value any) error {
	b, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(b, a)
}
