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
)

const (
	Iso8601Format     = "2006-01-02T15:04:05-0700"
	Iso8601DateFormat = "2006-01-02"
	ShortFormat       = "2006-01-02 15:04"
	JavascriptFormat  = "2006-01-02T15:04:05-07:00"
	JavascriptISO     = "2006-01-02T15:04:05.999Z"
)

// https://github.com/jinzhu/now -- interesting library for getting start of this minute etc

const Day = time.Hour * time.Duration(24)
const Week = Day * time.Duration(7)

type JSONTime time.Time

func (jt *JSONTime) UnmarshalJSON(raw []byte) error {
	s := strings.Trim(string(raw), "\"")
	t, err := time.Parse(time.RFC3339, s)
	if err == nil {
		*jt = JSONTime(t)
		return nil
	}
	t, err = ParseIso8601(s)
	if err == nil {
		*jt = JSONTime(t)
		return nil
	}
	return err
}

type SQLTime struct {
	time.Time
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

func ParseIso8601(str string) (t time.Time, e error) {
	if str != "" {
		t, e = time.Parse(Iso8601Format, str)
		if e != nil {
			t, e = time.Parse(Iso8601DateFormat, str)
		}
	}
	return
}

func GetHourAndAm(t time.Time, use24hour bool) (hour int, am bool) {
	hour = t.Hour()
	am = true
	if hour >= 12 {
		am = false
	}
	hour %= 12
	if hour == 0 {
		hour = 12
	}
	return
}

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

	//	zlog.Info("GetFloatingHour:", t, "base:", base, hour, "^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^")
	return hour
}

func IsBigTime(t time.Time) bool {
	return t.UTC() == BigTime
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
	if IsToday(t) {
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

func GetSecsAsHMSString(dur float64, sec bool, subdigits int) string {
	var parts []string

	h := int(dur) / 3600
	m := (int(dur) / 60)
	if h != 0 {
		parts = append(parts, fmt.Sprint(h))
	}
	if h != 0 {
		m %= 60
	}
	format := "%d"
	if h != 0 {
		format = "%02d"
	}
	parts = append(parts, fmt.Sprintf(format, m))
	if sec {
		s := dur
		if h != 0 || m != 0 {
			s = math.Mod(s, 60)
		}
		format := "%02"
		format += fmt.Sprintf(".%df", subdigits)
		parts = append(parts, fmt.Sprintf(format, s))
		// zlog.Info("GetSecsAsHMSString:", dur, subdigits, format, parts, h, m)
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

var BigTime = time.Date(2200, 01, 01, 0, 0, 0, 0, time.UTC) // time.Duration can max handle 290 years, so better than 3000

func ReplaceOldUnixTimeZoneNamesWithNew(name string) string {
	var names = map[string]string{
		"Africa/Asmera":                   "Africa/Asmara",
		"AKST9AKDT":                       "Aerica/Anchorage",
		"Africa/Timbuktu":                 "Africa/Bamako",
		"America/Argentina/omodRivadavia": "America/Argentina/Catamarca",
		"America/Atka":                    "America/Adak",
		"America/Buenos_Aires":            "America/Argentina/Buenos_Aires",
		"America/Catamarca":               "America/Argentina/Catamarca",
		"America/Coral_Harbour":           "America/Atikokan",
		"America/Cordoba":                 "America/Argentina/Cordoba",
		"America/Ensenada":                "America/Tijuana",
		"America/Fort_Wayne":              "America/Indiana/Indianapolis",
		"America/Indianapolis":            "America/Indiana/Indianapolis",
		"America/Jujuy":                   "America/Argentina/Jujuy",
		"America/Knox_IN":                 "America/Indiana/Knox",
		"America/Louisville":              "America/Kentucky/Louisville",
		"America/Mendoza":                 "America/Argentina/Mendoza",
		"America/Porto_Acre":              "America/Rio_Branco",
		"America/Rosario":                 "America/Argentina/Cordoba",
		"America/Virgin":                  "America/St_Thomas",
		"Asia/Ashkhabad":                  "Asia/Ashgabat",
		"Asia/Calcutta":                   "Asia/Kolkata",
		"Asia/Chungking":                  "Asia/Chongqing",
		"Asia/Dacca":                      "Asia/Dhaka",
		"Asia/Istanbul":                   "Europe/Istanbul",
		"Asia/Katmandu":                   "Asia/Kathmandu",
		"Asia/Macao":                      "Asia/Macau",
		"Asia/Saigon":                     "Asia/Ho_Chi_Minh",
		"Asia/Tel_Aviv":                   "Asia/Jerusalem",
		"Asia/Thimbu":                     "Asia/Thimphu",
		"Asia/Ujung_Pandang":              "Asia/Makassar",
		"Asia/Ulan_Bator":                 "Asia/Ulaanbaatar",
		"Atlantic/Faeroe":                 "Atlantic/Faroe",
		"Atlantic/Jan_Mayen":              "Europe/Oslo",
		"Australia/ACT":                   "Australia/Sydney",
		"Australia/Canberra":              "Australia/Sydney",
		"Australia/LHI":                   "Australia/Lord_Howe",
		"Australia/North":                 "Australia/Darwin",
		"Australia/NSW":                   "Australia/Sydney",
		"Australia/Queensland":            "Australia/Brisbane",
		"Australia/South":                 "Australia/Adelaide",
		"Australia/Tasmania":              "Australia/Hobart",
		"Australia/Victoria":              "Australia/Melbourne",
		"Australia/West":                  "Australia/Perth",
		"Australia/Yancowinna":            "Australia/Broken_Hill",
		"Brazil/Acre":                     "America/Rio_Branco",
		"Brazil/DeNoronha":                "America/Noronha",
		"Brazil/East":                     "America/Sao_Paulo",
		"Brazil/West":                     "America/Manaus",
		"Canada/Atlantic":                 "America/Halifax",
		"Canada/Central":                  "America/Winnipeg",
		"Canada/Eastern":                  "America/Toronto",
		"Canada/East-askatchewan":         "America/Regina",
		"Canada/Mountain":                 "America/Edmonton",
		"Canada/Newfoundland":             "America/St_Johns",
		"Canada/Pacific":                  "America/Vancouver",
		"Canada/Saskatchewan":             "America/Regina",
		"Canada/Yukon":                    "America/Whitehorse",
		"Chile/Continental":               "America/Santiago",
		"Chile/EasterIsland":              "Pacific/Easter",
		"Cuba":                            "Aerica/Havana",
		"Egypt":                           "Arica/Cairo",
		"Eire":                            "Erope/Dublin",
		"Etc/GMT":                         "UTC",
		"Etc/GMT+":                        "UTC",
		"Etc/UCT":                         "UTC",
		"Etc/Universal":                   "UTC",
		"Etc/UTC":                         "UTC",
		"Etc/Zulu":                        "UTC",
		"Europe/Belfast":                  "Europe/London",
		"Europe/Nicosia":                  "Asia/Nicosia",
		"Europe/Tiraspol":                 "Europe/Chisinau",
		"GB":                              "Erope/London",
		"GB-Eire":                         "Europe/London",
		"GMT":                             "UC",
		"GMT+0":                           "UTC",
		"GMT0":                            "UC",
		"GMT-0":                           "UTC",
		"Greenwich":                       "UC",
		"Hongkong":                        "Aia/Hong_Kong",
		"Iceland":                         "Alantic/Reykjavik",
		"Iran":                            "Aia/Tehran",
		"Israel":                          "Aia/Jerusalem",
		"Jamaica":                         "Aerica/Jamaica",
		"Japan":                           "Aia/Tokyo",
		"JST-9":                           "Asia/Tokyo",
		"Kwajalein":                       "Pcific/Kwajalein",
		"Libya":                           "Arica/Tripoli",
		"Mexico/BajaNorte":                "America/Tijuana",
		"Mexico/BajaSur":                  "America/Mazatlan",
		"Mexico/General":                  "America/Mexico_City",
		"Navajo":                          "Aerica/Denver",
		"NZ":                              "Pcific/Auckland",
		"NZ-CHAT":                         "Pacific/Chatham",
		"Pacific/Ponape":                  "Pacific/Pohnpei",
		"Pacific/Samoa":                   "Pacific/Pago_Pago",
		"Pacific/Truk":                    "Pacific/Chuuk",
		"Pacific/Yap":                     "Pacific/Chuuk",
		"Poland":                          "Erope/Warsaw",
		"Portugal":                        "Erope/Lisbon",
		"PRC":                             "Aia/Shanghai",
		"ROC":                             "Aia/Taipei",
		"ROK":                             "Aia/Seoul",
		"Singapore":                       "Aia/Singapore",
		"Turkey":                          "Europe/Istanbul",
		"UCT":                             "UC",
		"Universal":                       "UC",
		"US/Alaska":                       "America/Anchorage",
		"US/Aleutian":                     "America/Adak",
		"US/Arizona":                      "America/Phoenix",
		"US/Central":                      "America/Chicago",
		"US/Eastern":                      "America/New_York",
		"US/East-ndiana":                  "America/Indiana/Indianapolis",
		"US/Hawaii":                       "Pacific/Honolulu",
		"US/Indiana-tarke":                "America/Indiana/Knox",
		"US/Michigan":                     "America/Detroit",
		"US/Mountain":                     "America/Denver",
		"US/Pacific":                      "America/Los_Angeles",
		"US/Pacific-ew":                   "America/Los_Angeles",
		"US/Samoa":                        "Pacific/Pago_Pago",
		"W-SU":                            "Europe/Moscow",
		"Zulu":                            "UC",
	}
	newName := names[name]
	if newName != "" {
		return newName
	}
	return name
}

func GetStartOfToday(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func IsToday(t time.Time) bool {
	now := time.Now().In(t.Location())
	s := GetStartOfToday(now)
	e := s.Add(Day)
	if t.Sub(s) >= 0 && e.Sub(t) > 0 {
		return true
	}
	return false
}

func TimeZoneNameFromHourOffset(offset float32) string {
	// https://en.wikipedia.org/wiki/List_of_tz_database_time_zones
	switch offset {
	case -2.5, -3.5:
		return "America/St_Johns"
	case 5.5:
		return "Asia/Kolkata"
	case 3.5:
		return "Asia/Tehran"
	case 4.5:
		return "Asia/Kabul"
	case 5.75:
		return "Asia/Kathmandu"
	case 8.5:
		return "Asia/Pyongyang"
	case 6.5:
		return "Asia/Yangon"
	case 9.5, 10.5:
		return "Australia/Adelaide"
	}
	str := "Etc/GMT"
	offset *= -1 // Etc/GMT is posix, so reversed
	if offset > 0 {
		str += "+"
	}
	str += fmt.Sprintf("%d", int(offset))
	return str
}

func GetDurationHourMinSec(d time.Duration) (hours int, mins int, secs int) {
	s := int64(d.Seconds())
	hours = int(s / 3600)
	mins = int(s / 60 % 60)
	secs = int(s % 60)

	return
}

func GetDurationHourMinSecAsString(d time.Duration) (str string) {
	h, m, s := GetDurationHourMinSec(d)
	str = fmt.Sprintf("%02d:%02d", m, s)
	if h != 0 {
		str = fmt.Sprintf("%d:%s", h, str)
	}
	return
}

func GetClosestTo(n float64, to []float64) float64 {
	best := -1
	for i, t := range to {
		a := math.Abs(n - t)
		if best == -1.0 || a < math.Abs(n-to[best]) {
			best = i
		}
	}
	return to[best]
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
		i = GetClosestTo(i, []float64{1, 2, 3, 5, 10, 15, 20, 30})
	case time.Hour:
		i = GetClosestTo(i, []float64{1, 2, 3, 6, 12, 24})
	}
	u := start.Unix()
	ui := int64((time.Duration(i) * part) / time.Second)
	mod := u % ui
	n := u + (ui - mod)
	first = time.Unix(n, 0).In(start.Location())
	//	zlog.Info("best:", i, part, u, mod, n, first)
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

func DurationStructFromIso(dur string) (DurationStruct, error) {
	var regex = regexp.MustCompile(`P((?P<year>\d+)Y)?((?P<month>\d+)M)?((?P<day>\d+)D)?(T((?P<hour>\d+)H)?((?P<minute>\d+)M)?((?P<second>[\d\.]+)S)?)?`)
	var match []string

	d := DurationStruct{}

	if regex.MatchString(dur) {
		match = regex.FindStringSubmatch(dur)
	} else {
		return d, errors.New("bad format")
	}

	for i, name := range regex.SubexpNames() {
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

// RepeatFromNow invokes f immediately, and then at intervalSecs until the function returns false
func RepeatFromNow(intervalSecs float64, f func() bool) {
	ticker := time.NewTicker(SecondsDur(intervalSecs))
	if !f() {
		return
	}
	for range ticker.C {
		if !f() {
			ticker.Stop()
			break
		}
	}
}
