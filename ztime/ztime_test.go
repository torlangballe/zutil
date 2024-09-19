package ztime

import (
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

func parseDate(str string, use24 bool) string {
	t, _, err := ParseDate(str, time.UTC, use24)
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

	ztesting.Equal(t, parseDate("9:0 8-20", true), "big", "time/date causing problems")
	return
	ztesting.Equal(t, parseDate("banana", true), `strconv.Atoi: parsing "banana": invalid syntax`, "bad1")
	ztesting.Equal(t, parseDate("9:45", false), "no am/pm 9:45", "missing am/pm")
	ztesting.Equal(t, parseDate("9:60", true), date(10, 00, 0, day, month, year), "minute wrap")
	ztesting.Equal(t, parseDate("25:22", true), "big", "hour wrap")
	ztesting.Equal(t, parseDate("7:32am", false), date(7, 32, 0, day, month, year), "am/pm ok")
	ztesting.Equal(t, parseDate("7:32pm", false), date(19, 32, 0, day, month, year), "am/pm ok")
	ztesting.Equal(t, parseDate("13:45", true), date(13, 45, 0, day, month, year), "simple hour:min")
	ztesting.Equal(t, parseDate("4:54:21", true), date(4, 54, 21, day, month, year), "simple hour:min:sec")
	ztesting.Equal(t, parseDate("14:12 13", true), date(14, 12, 0, 13, month, year), "simple hour:min day")
	ztesting.Equal(t, parseDate("23:44t22", true), date(23, 44, 0, 22, month, year), "simple hour:mintday")
	ztesting.Equal(t, parseDate("22:45 14-7", true), date(22, 45, 0, 14, 7, year), "simple hour:min day-month")
	ztesting.Equal(t, parseDate("12-8", true), date(0, 0, 0, 12, 8, year), "simple day-month")
	ztesting.Equal(t, parseDate("30-8-2016", true), date(0, 0, 0, 30, 8, 2016), "simple day-month-year")
	ztesting.Equal(t, parseDate("5-5-18", true), date(0, 0, 0, 5, 5, 2018), "simple day-month-year")
	ztesting.Equal(t, parseDate("8:44:53pm 31-12-1939", false), date(20, 44, 53, 31, 12, 1939), "simple day-month-year")

}
