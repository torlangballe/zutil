//go:build server

package zsql

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// ALTER TABLE tbl_name ALTER COLUMN col_name TYPE varchar (11);

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
/*
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

type JSONStringMapForPtr map[string]string // we might not use this in future, move to JSONer?

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
	// zlog.Info("JSONStringInterfaceMap Value")
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *JSONStringInterfaceMap) Scan(val interface{}) error {
	//	zlog.Info("JSONStringInterfaceMap scan")
	if val == nil {
		*s = JSONStringInterfaceMap{}
		return nil
	}
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
*/

/* see zgeo.Pos, zgeo.

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
*/

func SetupPostgres(userName, dbName, address string) (db *sql.DB, err error) {
	pqStr := fmt.Sprintf(
		"host=%s port=%d sslmode=%s dbname=%s user=%s", //  password=%s
		address,
		5432,
		"disable",
		dbName,
		userName,
	)

	db, err = sql.Open("postgres", pqStr)
	zlog.Info("OPEN POSTGRES:", pqStr, err)
	if err != nil {
		zlog.Info("setup db err:", err)
		return
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)

	return
}

func PeriodicDump() {
	folder := "dumps/"
	ztimer.Repeat(60, func() bool {
		file := folder + "latest.db"
		if zfile.Exists(file) && time.Since(zfile.Modified(file)) > ztime.Day {
			zfile.MakeDirAllIfNotExists(folder)
			timeFile := time.Now().Format(ztime.ISO8601DateFormat) + ".db"
			os.Rename(file, timeFile)
			zlog.Info("Dump DB")
			zprocess.RunBashCommand("pg_dump etheros > "+file, 0)
		}
		return true
	})
}
