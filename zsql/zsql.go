package zsql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type (
	JSONer             []byte
	JSONStringArrayPtr []string
	JSONStringArray    []string
)

var replaceDollarRegex = regexp.MustCompile(`(\$[\d+])`) // Used by CustomizeQuery to find/replace $x

// CustomizeQuery can make a sqlite or psql query, replacing $x with > for sqlite.
// It also replaces $NOW and $PRIMARY-INT-INC with what is needed for each DB type.
func CustomizeQuery(query *string, isSqlite bool) {
	if isSqlite {
		*query = strings.Replace(*query, "$NOW", "CURRENT_TIMESTAMP", -1)
		*query = strings.Replace(*query, "$PRIMARY-INT-INC", "INTEGER PRIMARY KEY AUTOINCREMENT", -1)
		i := 1
		*query = zstr.ReplaceAllCapturesFunc(replaceDollarRegex, *query, func(cap string, index int) string {
			si, _ := strconv.Atoi(cap[1:])
			if si != i {
				zlog.Error(nil, "$x not right:", cap, i)
			}
			i++
			return "?"
		})
	} else {
		*query = strings.Replace(*query, "$NOW", "NOW()", -1)
		*query = strings.Replace(*query, "$PRIMARY-INT-INC", "SERIAL PRIMARY KEY", -1)
	}
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
