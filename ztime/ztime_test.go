package ztime

import (
	"fmt"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztesting"
)

func TestDaysSince2000FromTime(t *testing.T) {
	for day := 0; day < 12000; day++ {
		td := TimeOfDaysSince2000(day, nil)
		day2 := DaysSince2000FromTime(td)
		if day != day2 {
			t.Error("Different:", day, day2, td)
			return
		}
	}
}

func parseDate(str string, f TimeFieldFlags) string {
	t, _, err := ParseDate(str, time.UTC, f)
	if err != nil {
		return err.Error()
	}
	if t.IsZero() {
		return "zero"
	}
	return t.Format(time.RFC1123Z)
}

func date(hour, min, sec, day, month, year int) string {
	t := time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	return t.Format(time.RFC1123Z)
}

func TestParseDate(t *testing.T) {
	zlog.Warn("TestParseDate")

	now := time.Now().UTC()
	year := now.Year()
	month := int(now.Month())
	day := now.Day()

	ztesting.Equal(t, parseDate("9:0 8-20", TimeFieldNone), "big", "time/date causing problems")
	ztesting.Equal(t, parseDate("banana", TimeFieldNone), `strconv.Atoi: parsing "banana": invalid syntax`, "bad1")
	ztesting.Equal(t, parseDate("9:45", TimeFieldAMPM), "no am/pm 9:45", "missing am/pm")
	ztesting.Equal(t, parseDate("9:60", TimeFieldNone), date(10, 00, 0, day, month, year), "minute wrap")
	ztesting.Equal(t, parseDate("25:22", TimeFieldNone), "big", "hour wrap")
	ztesting.Equal(t, parseDate("7:32am", TimeFieldAMPM), date(7, 32, 0, day, month, year), "am/pm ok")
	ztesting.Equal(t, parseDate("7:32pm", TimeFieldAMPM), date(19, 32, 0, day, month, year), "am/pm ok")
	ztesting.Equal(t, parseDate("13:45", TimeFieldNone), date(13, 45, 0, day, month, year), "simple hour:min")
	ztesting.Equal(t, parseDate("4:54:21", TimeFieldNone), date(4, 54, 21, day, month, year), "simple hour:min:sec")
	ztesting.Equal(t, parseDate("14:12 13", TimeFieldNone), date(14, 12, 0, 13, month, year), "simple hour:min day")
	ztesting.Equal(t, parseDate("23:44t22", TimeFieldNone), date(23, 44, 0, 22, month, year), "simple hour:mintday")
	ztesting.Equal(t, parseDate("22:45 14-7", TimeFieldNone), date(22, 45, 0, 14, 7, year), "simple hour:min day-month")
	ztesting.Equal(t, parseDate("12-8", TimeFieldNone), date(0, 0, 0, 12, 8, year), "simple day-month")
	ztesting.Equal(t, parseDate("30-8-2016", TimeFieldNone), date(0, 0, 0, 30, 8, 2016), "simple day-month-year")
	ztesting.Equal(t, parseDate("5-5-18", TimeFieldNone), date(0, 0, 0, 5, 5, 2018), "simple day-month-year")
	ztesting.Equal(t, parseDate("8:44:53pm 31-12-1939", TimeFieldAMPM), date(20, 44, 53, 31, 12, 1939), "simple day-month-year")

	if day < 27 {
		str := fmt.Sprintf("16:35 %d", day+1)
		ztesting.Equal(t, parseDate(str, TimeFieldNotFutureIfAmbiguous), date(16, 35, 0, day+1, month-1, year), "ambiguous future1")
	}
	if month < 12 {
		str := fmt.Sprintf("16:35 %d-%d", day, month+1)
		ztesting.Equal(t, parseDate(str, TimeFieldNotFutureIfAmbiguous), date(16, 35, 0, day, month+1, year-1), "ambiguous future2")
	}
}
