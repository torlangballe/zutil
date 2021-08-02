package zsql

import (
	"fmt"
	"strings"
)

type JSONer []byte

type JSONStringArrayPtr []string
type JSONStringArray []string

func ReplaceQuestionMarkArguments(squery string, args ...interface{}) string {
	for _, a := range args {
		sa := fmt.Sprintf("'%v'", a)
		squery = strings.Replace(squery, "?", sa, 1)
	}
	return squery
}
