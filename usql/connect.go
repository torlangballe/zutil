package usql

import (
	"database/sql"
	"fmt"
	"github.com/torlangballe/zutil/ustr"

	"github.com/lib/pq"

	//	"os"
	"strings"
)

func MakeConnection(connString string, print bool) *sql.DB {
	//connString := os.Getenv("DATABASE_URL")

	parsedStr, err := pq.ParseURL(connString) // this parses a url to the variable-format
	if err == nil {
		//		fmt.Println("parsedStr:", parsedStr, err)
		connString = parsedStr
	}
	if print {
		fmt.Println(ustr.EscGreen+"Opening DB: ", strings.SplitN(connString, "password", 2)[0], ustr.EscWhite)
	}
	dbConn, err := sql.Open("postgres", connString)
	if err != nil {
		panic(err)
	}
	dbConn.Exec("SET TIME ZONE 'UTC'")
	return dbConn
}

func CloseConnection(dbConn *sql.DB) {
	err := dbConn.Close()
	if err != nil {
		panic(err)
	}
}
