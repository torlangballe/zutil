package zsql

import (
	"fmt"
	"strings"
	"time"
)

type JSONer []byte

type JSONStringArrayPtr []string
type JSONStringArray []string

func ReplaceQuestionMarkArguments(squery string, args ...interface{}) string {
	for _, a := range args {
		var sa string
		t, got := a.(time.Time)
		if got {
			sa = t.Format(time.RFC3339Nano)
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
