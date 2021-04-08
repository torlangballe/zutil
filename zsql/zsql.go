package zsql

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"

	"github.com/lib/pq"
)

type Executer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
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

func QuoteString(str string) string {
	return "'" + SanitizeString(str) + "'"
}

func SanitizeString(str string) string {
	return strings.Replace(str, "'", "''", -1)
}

func ConvertFieldName(i zreflect.Item) string {
	return strings.ToLower(i.FieldName)
}

func getItems(istruct interface{}, skip []string) (items []zreflect.Item) {
	options := zreflect.Options{UnnestAnonymous: true}
	all, _ := zreflect.ItterateStruct(istruct, options)
outer:
	for _, i := range all.Children {
		vars := zreflect.GetTagAsMap(i.Tag)["db"]
		if len(vars) != 0 && vars[0] == "-" {
			continue outer
		}
		if zstr.IndexOf(ConvertFieldName(i), skip) == -1 {
			//			zlog.Info("usql getItem:", i.FieldName)
			items = append(items, i)
		}
	}
	return
}

func FieldNamesFromStruct(istruct interface{}, skip []string, prefix string) (fields string) {
	for i, item := range getItems(istruct, skip) {
		if i != 0 {
			fields += ", "
		}
		fields += prefix + ConvertFieldName(item)
	}
	return
}

func FieldValuesFromStruct(istruct interface{}, skip []string) (values []interface{}) {
	for _, item := range getItems(istruct, skip) {
		v := item.Interface
		if item.Kind == zreflect.KindStruct && item.IsPointer {
			v = item.Address
		}
		if item.IsSlice {
			v = pq.Array(v)
		}
		values = append(values, v)
	}
	return
}

func FieldPointersFromStruct(istruct interface{}, skip []string) (pointers []interface{}) {
	for _, i := range getItems(istruct, skip) {
		a := i.Address
		if i.IsSlice {
			a = pq.Array(a)
		}
		// zlog.Info("FieldPointersFromStruct:", i.TypeName, i.Kind, i.FieldName, reflect.ValueOf(a).Type())
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
	zlog.Info("CreateSQLite3TableCreateStatementFromStruct", reflect.ValueOf(istruct).IsZero())
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
