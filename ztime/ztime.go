package ztime

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlocale"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zwords"
)

const (
	ISO8601Format       = "2006-01-02T15:04:05-0700"
	ISO8601NoZoneFormat = "2006-01-02T15:04:05"
	ISO8601DateFormat   = "2006-01-02"
	ShortFormat         = "2006-01-02 15:04"
	JavascriptFormat    = "2006-01-02T15:04:05-07:00"
	JavascriptISO       = "2006-01-02T15:04:05.999Z"
	FullCompact         = "06-Jan-02T15:04:05.9"
	NiceFormat          = "15:04:05 02-Jan-2006" // MaxSize of GetNice()

	Day  = time.Hour * time.Duration(24)
	Week = Day * time.Duration(7)
)

type JSONTime time.Time
type SQLTime struct {
	time.Time
}
type Differ time.Time

// Distant is a very far-future time when you want to do things forever etc
var (
	Distant                  = time.Unix(1<<62, 0)
	BigTime                  = time.Date(2200, 01, 01, 0, 0, 0, 0, time.UTC) // time.Duration can max handle 290 years, so better than 3000
	TestTime                 = MustParse("2020-05-17T10:30:45.0+02:00")      // Random time we can use in tests when it has to look normal and be a fixed time
	ServerTimezoneOffsetSecs int
	SundayFirstWeekdays      = []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}
	Weekdays                 = []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday, time.Sunday}
)

// https://github.com/jinzhu/now -- interesting library for getting start of this minute etc

func NewDiffer() Differ {
	return Differ(time.Now())
}

func (d Differ) String() string {
	return time.Since(time.Time(d)).String()
}

func (jt *JSONTime) UnmarshalJSON(raw []byte) error {
	s := strings.Trim(string(raw), "\"")
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		*jt = JSONTime(t)
		return nil
	}
	t, err = ParseISO8601(s)
	if err == nil {
		*jt = JSONTime(t)
		return nil
	}
	return err
}

func (t *SQLTime) Scan(value interface{}) error {
	var valid bool
	t.Time, valid = value.(time.Time)
	if !valid {
		t.Time = time.Time{}
	}
	return nil
}

func (t SQLTime) Value() (driver.Value, error) {
	if !t.IsZero() {
		return nil, nil
	}
	return t.Time, nil
}

func Make(t time.Time) SQLTime {
	var st SQLTime
	st.Time = t

	return st
}

func ParseISO8601(str string) (t time.Time, e error) {
	if str != "" {
		t, e = time.Parse(ISO8601Format, str)
		if e != nil {
			t, e = time.Parse(ISO8601DateFormat, str)
		}
	}
	return
}

func GetHourAndAM(t time.Time) (hour int, am bool) {
	hour = t.Hour()
	am = hour < 12
	hour %= 12
	if hour == 0 {
		hour = 12
	}
	return
}

/*
// GetFloatingHour returns the hour, min, secs as hours with a decimal fraction

	func GetFloatingHour(t, base time.Time) float64 {
		h := t.Hour()
		m := t.Minute()
		s := t.Second()
		if !base.IsZero() {
			_, _, tday := t.Date()
			_, _, bday := base.Date()
			if tday != bday {
				if base.After(t) {
					h -= 24
				} else {
					h += 24
				}
			}
		}
		hour := float64(h) + float64(m)/60.0 + float64(s)/3600.0
		return hour
	}
*/

func IsBigTime(t time.Time) bool {
	return t.UTC() == BigTime
}

func GetTimeWithServerLocation(t time.Time) time.Time {
	if !zlocale.DisplayServerTime.Get() {
		//		zlog.Info("GetTimeWithServerLocation", t, ServerTimezoneOffsetSecs, t.Location())
		t = t.Local()
		return t
	}
	// zlog.Info("GetTimeWithServerLocation", t, ServerTimezoneOffsetSecs)
	name := fmt.Sprintf("UTC%+f", float64(ServerTimezoneOffsetSecs)/3600)
	loc := time.FixedZone(name, ServerTimezoneOffsetSecs)
	return t.In(loc)
}

func GetNice(t time.Time, secs bool) string {
	var str string
	if t.IsZero() {
		return "null"
	}
	if IsBigTime(t) {
		return "∞"
	}

	f := "15:04"
	if secs {
		f += ":05"
	}
	serverTime := zlocale.DisplayServerTime.Get()
	if serverTime {
		f += "-07"
	}
	t = GetTimeWithServerLocation(t)
	if IsToday(t) && !serverTime {
		str = t.Format(f) + " today"
	} else {
		f += " 02-Jan"
		if t.Year() != time.Now().Year() {
			f += " 2006"
		}
		str = t.Format(f)
	}
	return str
}

func GetShort(t time.Time) string {
	if t.IsZero() {
		return "null"
	}
	if IsBigTime(t) {
		return "∞"
	}
	format := "02-Jan-2006 15:04"
	str := t.Format(format)
	return str
}

func Minutes(m int) time.Duration {
	return time.Duration(m) * time.Minute
}

func SecondsDur(s float64) time.Duration {
	return time.Duration(s * float64(time.Second))
}

func DurSeconds(d time.Duration) float64 {
	return float64(d) / float64(time.Second)
}

func Since(t time.Time) float64 {
	return DurSeconds(time.Since(t))
}

func GetDurationAsHMSString(duration time.Duration, hours, mins, secs bool, subdigits int) string {
	var parts []string
	durSecs := DurSeconds(duration)
	h := int(durSecs) / 3600
	m := (int(durSecs) / 60)
	if hours && h != 0 {
		parts = append(parts, fmt.Sprint(h))
	}
	if h != 0 {
		m %= 60
	}
	format := "%d"
	if h != 0 {
		format = "%02d"
	}
	if mins {
		parts = append(parts, fmt.Sprintf(format, m))
	}
	if secs {
		s := durSecs
		if h != 0 || m != 0 {
			s = math.Mod(s, 60)
		}
		format := "%02"
		format += fmt.Sprintf(".%df", subdigits)
		parts = append(parts, fmt.Sprintf(format, s))
		// zlog.Info("GetSecsAsHMSString:", durSecs, subdigits, format, parts, h, m)
	}
	return strings.Join(parts, ":")
}

func GetSecsFromHMSString(str string, hour, min, sec bool) (float64, error) {
	var secs float64

	parts := strings.Split(str, ":")
	for _, p := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return 0, err
		}
		if hour {
			secs += n * 3600
			hour = false
		} else if min {
			secs += n * 60
			min = false
		} else if sec {
			secs += n
			sec = false
		}
	}
	return secs, nil
}

// MustParse is ued to parse a RFC3339Nano time from string, typically for using in literals.
func MustParse(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	zlog.AssertNotError(err)
	return t
}

func WithIntegerSeconds(t time.Time) time.Time {
	n := t.Unix()
	return time.Unix(n, 0).In(t.Location())
}

func GetStartOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func GetStartOfToday() time.Time {
	return GetStartOfDay(time.Now().Local())
}

func IsSameDay(a, b time.Time) bool {
	if a.Year() != b.Year() {
		return false
	}
	if a.Month() != b.Month() {
		return false
	}
	if a.Day() != b.Day() {
		return false
	}
	return true
}

// IsToday returns true if t in local time is same day as now local.
func IsToday(t time.Time) bool {
	return IsSameDay(t.Local(), time.Now())
}

func GetDurationHourMinSec(d time.Duration) (hours int, mins int, secs int, fract float64) {
	ds := d.Seconds()
	s := int64(ds)
	hours = int(s / 3600)
	mins = int(s / 60 % 60)
	secs = int(s % 60)
	fract = ds - float64(s)
	return
}

func GetDurNice(d time.Duration, fractDigits int) string {
	var parts []string
	h, m, s, f := GetDurationHourMinSec(d)
	if h != 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}
	if m != 0 {
		parts = append(parts, fmt.Sprintf("%dm", m))
	}
	if s != 0 {
		if fractDigits == 0 {
			parts = append(parts, fmt.Sprintf("%ds", s))
		} else {
			parts = append(parts, zwords.NiceFloat(float64(s)+f, fractDigits))
		}
	}
	return strings.Join(parts, " ")
}

func GetDurationString(d time.Duration, secs, mins, hours bool, subDigits int) (str string, overflow bool) {
	h, m, s, fract := GetDurationHourMinSec(d)
	if hours {
		str = fmt.Sprint(h)
	} else if h > 0 {
		overflow = true
	}
	if mins {
		str = zstr.Concat(":", str, fmt.Sprintf("%02d", m))
	} else if m > 0 {
		overflow = true
	}
	if secs {
		var ss string
		if subDigits > 0 {
			ss = zwords.NiceFloat(float64(s)+fract, subDigits)
		} else {
			ss = fmt.Sprint(s)
		}
		if s < 10 && mins {
			ss = "0" + ss
		}
		str = zstr.Concat(":", str, ss)
	}
	return
}

func GetNiceIncsOf(start, stop time.Time, incCount int) (inc time.Duration, first time.Time) {
	diff := stop.Sub(start)
	parts := []time.Duration{time.Second, time.Minute, time.Hour, Day}

	bi := -1
	logInc := math.Log(float64(incCount))
	var best float64
	for i, p := range parts {
		mult := float64(diff) / (float64(p) * float64(incCount))
		log := math.Log(mult)
		delta := math.Abs(log - logInc)
		//		zlog.Info(i, incCount, mult, delta, log, logInc)
		if bi == -1 || delta < best {
			best = delta
			bi = i
		}
	}
	part := parts[bi]
	bestPart := float64(diff) / float64(part)
	i := math.Max(1.0, math.Round(bestPart/float64(incCount)))
	switch part {
	case time.Second, time.Minute:
		i = zmath.GetClosestTo(i, []float64{1, 2, 5, 10, 15, 20, 30})
	case time.Hour:
		i = zmath.GetClosestTo(i, []float64{1, 2, 3, 6, 12, 24})
	}
	u := start.Unix()
	ui := int64((time.Duration(i) * part) / time.Second)
	mod := u % ui
	n := u + (ui - mod)
	first = time.Unix(n, 0).In(start.Location())
	// zlog.Info("best:", i, part, u, mod, n, first)
	inc = time.Duration(i) * part
	return
}

type DurationStruct struct {
	Years   int
	Weeks   int
	Months  int
	Days    int
	Hours   int
	Minutes int
	Seconds float32
}

var ISODurRegEx = regexp.MustCompile(`P((?P<year>\d+)Y)?((?P<month>\d+)M)?((?P<day>\d+)D)?(T((?P<hour>\d+)H)?((?P<minute>\d+)M)?((?P<second>[\d\.]+)S)?)?`)

// DurationStructFromISO reads an ISO 8601 dduration string.
func DurationStructFromISO(dur string) (DurationStruct, error) {
	// TODO: Make regex stuff more optimal?
	var match []string

	d := DurationStruct{}

	if ISODurRegEx.MatchString(dur) {
		match = ISODurRegEx.FindStringSubmatch(dur)
	} else {
		return d, errors.New("bad format")
	}

	for i, name := range ISODurRegEx.SubexpNames() {
		part := match[i]
		if i == 0 || name == "" || part == "" {
			continue
		}

		val, err := strconv.ParseFloat(part, 32)
		if err != nil {
			return d, err
		}
		switch name {
		case "year":
			d.Years = int(val)
		case "month":
			d.Months = int(val)
		case "week":
			d.Weeks = int(val)
		case "day":
			d.Days = int(val)
		case "hour":
			d.Hours = int(val)
		case "minute":
			d.Minutes = int(val)
		case "second":
			d.Seconds = float32(val)
		default:
			return d, errors.New(fmt.Sprintf("unknown field %s", name))
		}
	}

	return d, nil
}

func (d DurationStruct) ToDuration() time.Duration {
	day := time.Hour * 24
	year := day * 365

	tot := time.Duration(0)

	tot += year * time.Duration(d.Years)
	tot += day * 7 * time.Duration(d.Weeks)
	tot += day * 30 * time.Duration(d.Months)
	tot += day * time.Duration(d.Days)
	tot += time.Hour * time.Duration(d.Hours)
	tot += time.Minute * time.Duration(d.Minutes)
	tot += time.Second * time.Duration(d.Seconds)

	return tot
}

func MonthFromString(str string) (time.Month, bool) {
	str = strings.ToLower(str)
	is3 := (len(str) == 3)
	fmt.Println("MonthFromString:", str, is3)
	for m := time.January; m <= time.December; m++ {
		n := strings.ToLower(m.String())
		if is3 {
			if n[:3] == str {
				return m, true
			}
		} else {
			if n == str {
				return m, true
			}
		}
	}
	return 0, false
}

func DurationFromString(str string) time.Duration {
	var sh, sm, ss string
	if zstr.SplitN(str, ":", &sh, &sm, &ss) {
		h, _ := strconv.ParseInt(sh, 10, 64)
		m, _ := strconv.ParseInt(sm, 10, 64)
		s, _ := strconv.ParseInt(ss, 10, 64)
		return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second
	}
	return 0
}

func Maximize(t *time.Time, with time.Time) {
	if with.Sub(*t) > 0 {
		*t = with
	}
}

func Minimize(t *time.Time, with time.Time) {
	if with.Sub(*t) < 0 {
		*t = with
	}
}

func XToTime(minX, maxX float64, x float64, start, end time.Time) time.Time {
	tdiff := DurSeconds(end.Sub(start))
	dur := SecondsDur((x - minX) / (maxX - minX) * tdiff)
	return start.Add(dur)
}

func TimeToX(minX, maxX float64, t, start, end time.Time) float64 {
	diff := DurSeconds(end.Sub(start))
	return minX + DurSeconds(t.Sub(start))*(maxX-minX)/diff
}

// AddMonthAndYearToTime adds months and years to a date.
// It increases/decreases month-count, not using 30 days.
// If the results day of year is outside current month it is truncated.
func AddMonthAndYearToTime(t time.Time, months, years int) time.Time {
	year := t.Year()
	month := t.Month()
	day := t.Day()
	year += years
	month += time.Month(months)
	max := DaysInMonth(month, year)
	zint.Minimize(&day, max)
	n := time.Date(year, month, day, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
	return n
}

// DaysInMonth returns the number of days in a month for a given year
func DaysInMonth(month time.Month, year int) int {
	t := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC) // 0 of next month is the same as last day of given month
	return t.Day()
}

func OnTheNextHour(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 60, 0, 0, t.Location())
}

func DaysSince2000FromTime(t time.Time) int {
	year, month, day := t.Date()
	years := year - 2000
	days := years * 365
	leaps := (years + 3) / 4 // 2000 is leap year. Don't count day's year's leap year. Good until 2100.
	days += leaps
	var mdays int
	for m := time.January; m < month; m++ {
		// zlog.Warn("month", m, DaysInMonth(m, year))
		mdays += DaysInMonth(m, year)
	}
	days += mdays
	days += day - 1
	// zlog.Warn("years", years*365)
	// zlog.Warn("leaps", leaps)
	// zlog.Warn("yeardays", mdays+day-1)
	// zlog.Warn("mdays", mdays)
	// zlog.Warn("days", day-1)
	return days
}

func TimeOfDaysSince2000(days int, loc *time.Location) time.Time {
	if loc == nil {
		loc = time.Local
	}
	t2000 := time.Date(2000, time.January, 1, 0, 0, 0, 0, loc)
	t := t2000.AddDate(0, 0, days)
	return t
}
