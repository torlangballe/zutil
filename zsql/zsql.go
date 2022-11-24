package zsql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type (
	JSONer             []byte
	JSONStringArrayPtr []string
	JSONStringArray    []string
)

type UpsertInfo struct {
	Rows         SQLDictSlice
	TableName    string
	SetColumns   map[string]string
	EqualColumns map[string]string
	OffsetQuery  string
}

type UpsertResult struct {
	LastInsertID int64
	Offset       int64
}

type SelectInfo struct {
	TableName  string
	GetColumns []string
	Trailer    string
}

type BaseType int

const (
	SQLite BaseType = iota + 1
	Postgres
	MySQL
)

var replaceDollarRegex = regexp.MustCompile(`(\$[\d+])`) // Used by CustomizeQuery to find/replace $x

// CustomizeQuery can make a sqlite or psql (or other in future) query, replacing $x with > for sqlite.
// It also replaces $NOW, $SERIAL and $PRIMARY-INT-INC with what is needed for each DB type.
func CustomizeQuery(query string, btype BaseType) string {
	switch btype {
	case SQLite:
		query = strings.Replace(query, "$SERIAL", "BIGINT", -1)
		query = strings.Replace(query, "$NOW", "CURRENT_TIMESTAMP", -1)
		query = strings.Replace(query, "$PRIMARY-INT-INC", "INTEGER PRIMARY KEY AUTOINCREMENT", -1)
		i := 1
		query = zstr.ReplaceAllCapturesFunc(replaceDollarRegex, query, func(cap string, index int) string {
			si, _ := strconv.Atoi(cap[1:])
			if si != i {
				zlog.Error(nil, "$x not right:", cap, i)
			}
			i++
			return "?"
		})
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

func FieldNamesStringFromStruct(istruct interface{}, skip []string, prefix string) (fields string) {
	fs := FieldNamesFromStruct(istruct, skip, prefix)
	return strings.Join(fs, ", ")
}

func FieldNamesFromStruct(istruct interface{}, skip []string, prefix string) (fields []string) {
	for _, item := range getItems(istruct, skip) {
		fields = append(fields, prefix+ConvertFieldName(item))
	}
	return
}
