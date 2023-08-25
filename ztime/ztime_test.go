package ztime

import (
	"testing"
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
