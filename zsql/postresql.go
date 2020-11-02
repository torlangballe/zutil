package zsql

import (
	"bytes"
	"capsulefm/libs/util/ugeo"
	"database/sql"
	"database/sql/driver"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	geom "github.com/twpayne/go-geom"
	wkb "github.com/twpayne/go-geom/encoding/wkb"
)

// type RawString string

// var ZeroTimeString = "0001-01-01 00:00:00"

func ReplaceDollarArguments(squery string, args ...interface{}) string {
	var to string
	for i, a := range args {
		from := fmt.Sprintf("$%d", i+1)
		switch a.(type) {
		case string, time.Time:
			to = fmt.Sprintf("'%v'", a)
		default:
			to = fmt.Sprintf("%v", a)
		}
		squery = strings.Replace(squery, from, to, -1)
	}
	return squery
}

// func GetOrFromInt64Slice(ids []int64, varName string) (str string) {
// 	for i, id := range ids {
// 		if i != 0 {
// 			str += " OR "
// 		}
// 		str += fmt.Sprintf("%s=%d", varName, id)
// 	}
// 	if str != "" {
// 		str = "(" + str + ")"
// 	}
// 	return
// }

type JSONStringArrayPtr []string

func (s *JSONStringArrayPtr) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringArrayPtr) Scan(val interface{}) error {
	data, ok := val.([]byte)
	if !ok {
		return errors.New("JSONStringArrayPtr unsupported data type")
	}
	return json.Unmarshal(data, s)
}

func (s *JSONStringArrayPtr) Join(sep string) string {
	if s == nil {
		return ""
	}
	return strings.Join(*s, sep)
}

type JSONStringArray []string

func (s JSONStringArray) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringArray) Scan(val interface{}) error {
	data, ok := val.([]byte)
	if !ok {
		return nil
		//		zlog.Info("JSONStringArray unsupported data type:", val)
		//      return errors.New("JSONStringArray unsupported data type")
	}
	return json.Unmarshal(data, s)
}

type JSONStringMapForPtr map[string]string

func (s *JSONStringMapForPtr) GetFromLangKeyOrEng(lang, def string) string {
	if s == nil {
		return def
	}
	str, got := (*s)[lang]
	if got {
		return str
	}
	str, got = (*s)["en"]
	if got {
		return str
	}
	return def
}

func (s *JSONStringMapForPtr) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringMapForPtr) Scan(val interface{}) error {
	data, ok := val.([]byte)
	if !ok {
		return errors.New("JSONStringArrayPtr unsupported data type")
	}
	return json.Unmarshal(data, s)
}

type JSONStringInterfaceMap map[string]interface{}

func (s JSONStringInterfaceMap) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringInterfaceMap) Scan(val interface{}) error {
	//	zlog.Info("JSONStringInterfaceMap scan")
	data, ok := val.([]byte)
	if !ok {
		return errors.New("JSONStringInterfaceMap unsupported data type")
	}
	return json.Unmarshal(data, s)
}

type JSONStringMap map[string]string

func (s JSONStringMap) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringMap) Scan(val interface{}) error {
	data, ok := val.([]byte)
	if !ok {
		return errors.New("JSONStringMap unsupported data type")
	}
	return json.Unmarshal(data, s)
}

// GisGeoPoint maps against Postgis geographical Point
type GisGeoPoint struct {
	Y float64 `json:"y"`
	X float64 `json:"x"`
}

func (p *GisGeoPoint) String() string {
	//return fmt.Sprintf("ST_GeomFromText('POINT(%v %v)', 4326)", p.Lat, p.Lng)
	return fmt.Sprintf("POINT(%v %v)", p.X, p.Y)
}

func scanFloat64(data []byte, littleEndian bool) float64 {
	var v uint64

	if littleEndian {
		for i := 7; i >= 0; i-- {
			v <<= 8
			v |= uint64(data[i])
		}
	} else {
		for i := 0; i < 8; i++ {
			v <<= 8
			v |= uint64(data[i])
		}
	}

	return math.Float64frombits(v)
}

func scanPrefix(data []byte) (bool, uint32, error) {
	if len(data) < 6 {
		return false, 0, errors.New("Not WKB4")
	}
	if data[0] == 0 {
		return false, scanUint32(data[1:5], false), nil
	}
	if data[0] == 1 {
		return true, scanUint32(data[1:5], true), nil
	}
	zlog.Info("scanPrefix data[0]:", data[0])
	return false, 0, errors.New("Not WKB5")
}

func scanUint32(data []byte, littleEndian bool) uint32 {
	var v uint32

	if littleEndian {
		for i := 3; i >= 0; i-- {
			v <<= 8
			v |= uint32(data[i])
		}
	} else {
		for i := 0; i < 4; i++ {
			v <<= 8
			v |= uint32(data[i])
		}
	}

	return v
}

func (p *GisGeoPoint) Scan(val interface{}) error {
	b, ok := val.([]byte)
	if !ok {
		return errors.New("GisGeoPoint unsupported data type")
	}
	data, err := hex.DecodeString(string(b))
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}
	if len(data) != 21 {
		// the length of a point type in WKB
		return errors.New(fmt.Sprintln("GisGeoPoint unsupported data length", len(data)))
	}

	littleEndian, typeCode, err := scanPrefix(data)
	if err != nil {
		return err
	}

	if typeCode != 1 {
		return errors.New("GisGeoPoint incorrect type for Point (not 1)")
	}

	p.X = scanFloat64(data[5:13], littleEndian)
	p.Y = scanFloat64(data[13:21], littleEndian)

	return nil
}

// Value implements the driver Valuer interface and will return the string representation of the GisGeoPoint struct by calling the String() method
func (p *GisGeoPoint) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	str := p.String()
	//	zlog.Info("GisGeoPoint Value:", str)
	return []byte(str), nil
}

// GisGeoPoint maps against Postgis geographical Point
type GisGeoPolygon [][]ugeo.FPoint

func (gp *GisGeoPolygon) Scan(val interface{}) error {
	b, ok := val.([]byte)
	if !ok {
		return wkb.ErrExpectedByteSlice{Value: val}
	}
	data, err := hex.DecodeString(string(b))
	if err != nil {
		return err
	}
	got, err := wkb.Unmarshal(data)
	if err != nil {
		return err
	}
	mp1, ok := got.(*geom.MultiPolygon)

	if !ok {
		return errors.New("GisGeoPolygon.Scan type not multipoly")
	}
	for _, npoly := range mp1.Coords() {
		for _, poly := range npoly {
			var sp []ugeo.FPoint
			for _, p := range poly {
				if len(p) == 2 {
					sp = append(sp, ugeo.FPoint{p[0], p[1]})
				}
			}
			*gp = append(*gp, sp)
		}
	}
	return nil
}

func (p *GisGeoPolygon) Value() (driver.Value, error) {
	//	zlog.Info("GisGeoPolygon.Value: EMPTY")
	if p == nil || len(*p) == 0 {
		return nil, nil
	}

	buff := bytes.NewBuffer(nil)
	fmt.Fprint(buff, "MULTIPOLYGON(")

	for pi, polys := range *p {
		if pi != 0 {
			fmt.Fprint(buff, ", ")
		}
		fmt.Fprint(buff, "((")
		for i, pos := range polys {
			if i != 0 {
				fmt.Fprint(buff, ",")
			}
			fmt.Fprintf(buff, "%g %g", pos.X, pos.Y)
		}
		if len(polys) > 1 {
			fmt.Fprintf(buff, ", %g %g", polys[0].X, polys[0].Y)
		}
		fmt.Fprint(buff, "))")
	}
	fmt.Fprint(buff, ")")
	data := buff.Bytes()

	//	zlog.Info("GisGeoPolygon.Value:", string(data))
	return data, nil
}

func MakeNDollarParametersInBrackets(n int, start int) string {
	str := "("
	for i := 0; i <= n; i++ {
		str += fmt.Sprint("$%d", n+start)
		if i != n {
			str += ","
		}
	}
	return str + ")"
}

/*
func printRows(writer io.Writer, rows *sql.Rows, limitWidth bool) {
	quit := false
	cols, _ := rows.Columns()
	header := "" // zstr.EscGreen)
	for i, c := range cols {
		if i != 0 {
			header += "\t"
		}
		header += c
	}
	fmt.Fprintln(writer, header)
	maxWidth := 200 / len(cols)
	for rows.Next() && !quit {
		var values []interface{}
		var generic = reflect.TypeOf(values).Elem()
		for i := 0; i < len(cols); i++ {
			values = append(values, reflect.New(generic).Interface())
		}
		rows.Scan(values...)
		for i := 0; i < len(cols); i++ {
			var raw_value = *(values[i].(*interface{}))
			switch reflect.TypeOf(raw_value) {
			case reflect.TypeOf([]byte{}):
				str := string(raw_value.([]uint8))
				if limitWidth {
					str = zstr.Head(str, maxWidth)
				}
				fmt.Fprintf(writer, "%s\t", str)
			case reflect.TypeOf(time.Time{}):
				if raw_value.(time.Time).IsZero() {
					fmt.Fprint(writer, "0\t")
				} else {
					fmt.Fprintf(writer, "%s\t", raw_value.(time.Time).Local().Format("2006-01-02T15:04:05"))
				}
			default:
				fmt.Fprintf(writer, "%v\t", raw_value)
			}
		}
		fmt.Fprintln(writer, "")
	}
}
*/

func SetupPostgres(userName, dbName string) (db *sql.DB, err error) {
	pqStr := fmt.Sprintf(
		"host=%s port=%d sslmode=%s dbname=%s user=%s", //  password=%s
		"127.0.0.1",
		5432,
		"disable",
		dbName,
		userName,
	)

	db, err = sql.Open("postgres", pqStr)
	zlog.Info("OPENPSQL:", pqStr, err)
	if err != nil {
		zlog.Info("setup db err:", err)
		return
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return
}

/*
func MakeConnection(connString string, print bool) *sql.DB {
	//connString := os.Getenv("DATABASE_URL")

	parsedStr, err := pq.ParseURL(connString) // this parses a url to the variable-format
	if err == nil {
		//		zlog.Info("parsedStr:", parsedStr, err)
		connString = parsedStr
	}
	if print {
		zlog.Info(zstr.EscGreen+"Opening DB: ", strings.SplitN(connString, "password", 2)[0], zstr.EscWhite)
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
*/
