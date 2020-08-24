package ztime

import (
	"errors"
	"fmt"
	"log"
	"time"
	"github.com/torlangballe/zutil/zstr"
)

type TimePreposition uint

const (
	Undef TimePreposition = iota
	KAt
	KUntil
	KFrom
)

type TimeParameters struct {
	Preposition           TimePreposition
	TimeStr, LangCode     string
	Use24Hour, IsAccurate bool
	ListenerTimeZone      string
	ShowDate, ShowTime    bool
	UseTimeStrZone        bool
}

const (
	Today            = 0
	Tomorrow         = 1
	DayAfterTomorrow = 2
	Later            = 10
	Yesterday        = -1
	//	DayBeforeYestrday = -2
	Earlier = -10
)

var hourName = map[string]string{
	"no": "time",
	"da": "time",
	"sv": "time",
	"de": "Stunde",
	"en": "hour",
}

var minuteName = map[string]string{
	"no": "minutt",
	"da": "minutt",
	"sv": "minut",
	"de": "Minute",
	"en": "minute",
}

var andName = map[string]string{
	"no": "og",
	"da": "og",
	"sv": "och",
	"de": "und",
	"en": "and",
}

type LangValue struct {
	LangCode string
	Value    int
}

var dayPosName = map[LangValue]string{
	LangValue{"en", Today}:            "today",
	LangValue{"no", Today}:            "idag",
	LangValue{"de", Today}:            "heute",
	LangValue{"en", Tomorrow}:         "tomorrow",
	LangValue{"no", Tomorrow}:         "imorgen",
	LangValue{"de", Tomorrow}:         "morgen",
	LangValue{"en", DayAfterTomorrow}: "the day after tomorrow",
	LangValue{"no", DayAfterTomorrow}: "i overimorgen",
	LangValue{"de", DayAfterTomorrow}: "Übermorgen",
	LangValue{"en", Yesterday}:        "yesterday",
	LangValue{"no", Yesterday}:        "i går",
	LangValue{"de", Yesterday}:        "gestern",
	//	LangValue{"de", DayBeforeYestrday}: "vorgestern",
	//	LangValue{"no", DayBeforeYestrday}: "i forigårs",
}

var weekdayNamePreposition = map[string]string{
	"en": "on",
	"no": "på",
	"de": "am",
}

const (
	Midnight = iota
	MorningNight
	Morning
	Fornoon
	MidDay
	Afternoon
	Evening
	Night
)

func getDayPeriodFromHour(hour int) int {
	switch hour {
	case 22, 23:
		return Night
	case 1, 2, 3, 4:
		return MorningNight
	case 5, 6, 7, 8, 9:
		return Morning
	case 10, 11:
		return Fornoon
	case 12:
		return MidDay
	case 13, 14, 15, 16:
		return Afternoon
	case 17, 18, 19, 20, 21:
		return Evening
	}
	return Midnight
}

var daytimePeriodFormat = map[LangValue]string{
	LangValue{"en", Midnight}:     "%s night",
	LangValue{"en", MorningNight}: "%s morning",
	LangValue{"en", Morning}:      "%s morning",
	LangValue{"en", Fornoon}:      "%s morning",
	LangValue{"en", MidDay}:       "%s mid-day",
	LangValue{"en", Afternoon}:    "%s afternoon",
	LangValue{"en", Evening}:      "%s evening",
	LangValue{"en", Night}:        "%s night",

	LangValue{"no", Midnight}:     "natt til %s",
	LangValue{"no", MorningNight}: "natt til %s",
	LangValue{"no", Morning}:      "%s morgen",
	LangValue{"no", Fornoon}:      "%s formiddag",
	LangValue{"no", MidDay}:       "%s ettermiddag",
	LangValue{"no", Afternoon}:    "%s ettermiddag",
	LangValue{"no", Evening}:      "%s kveld",
	LangValue{"no", Night}:        "%s natt",

	LangValue{"de", Midnight}:     "in der Nacht, %s",
	LangValue{"de", MorningNight}: "in der Nacht, %s",
	LangValue{"de", Morning}:      "am Morgen, %s",
	LangValue{"de", Fornoon}:      "am Vormittag, %s",
	LangValue{"de", MidDay}:       "am Mittag, %s",
	LangValue{"de", Afternoon}:    "am Nachmittag, %s",
	LangValue{"de", Evening}:      "am Abend, %s",
	LangValue{"de", Night}:        "am Abend, %s",
}

var todayPeriodExpression = map[LangValue]string{
	LangValue{"en", Midnight}:     "at night",
	LangValue{"en", MorningNight}: "this morning",
	LangValue{"en", Morning}:      "this morning",
	LangValue{"en", Fornoon}:      "this morning",
	LangValue{"en", MidDay}:       "mid-day",
	LangValue{"en", Afternoon}:    "this afternoon",
	LangValue{"en", Evening}:      "this evening",
	LangValue{"en", Night}:        "tonight",

	LangValue{"no", Midnight}:     "i natt",
	LangValue{"no", MorningNight}: "i natt",
	LangValue{"no", Morning}:      "imorges",
	LangValue{"no", Fornoon}:      "i formiddag",
	LangValue{"no", MidDay}:       "i dag",
	LangValue{"no", Afternoon}:    "i ettermiddag",
	LangValue{"no", Evening}:      "i kveld",
	LangValue{"no", Night}:        "i kveld",

	LangValue{"de", Midnight}:     "Nachts",
	LangValue{"de", MorningNight}: "Nachts",
	LangValue{"de", Morning}:      "Morgens",
	LangValue{"de", Fornoon}:      "Vormittags",
	LangValue{"de", MidDay}:       "Mittags",
	LangValue{"de", Afternoon}:    "Nachmittags",
	LangValue{"de", Evening}:      "Abends",
	LangValue{"de", Night}:        "Abends",
}

var localTimeExpression = map[string]string{
	"no": "lokal tid",
	"en": "local time",
	"de": "Ortszeit",
}

func GetRelativeDay(now, then time.Time) int {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	if then.After(today.AddDate(0, 0, 1)) {
		if then.After(today.AddDate(0, 0, 2)) {
			if then.After(today.AddDate(0, 0, 3)) {
				return Later
			}
			return DayAfterTomorrow
		}
		return Tomorrow
	}
	if then.Before(today) {
		if then.Before(today.AddDate(0, 0, -1)) {
			//			if then.Before(today.AddDate(0, 0, -2)) {
			return Earlier
			//			}
			//			return DayBeforeYestrday
		}
		return Yesterday
	}
	return Today
}

func getRoundedMinutes(m int) int {
	if m > 56 {
		return 55
	}
	return ((m + 3) / 5) * 5
}

func getPreposition(tp TimeParameters) string {
	switch tp.LangCode {
	case "en":
		if tp.Preposition != Undef {
			if tp.ShowTime {
				return "at"
			}
			return "on"
		}
	case "no":
		return ""
	}
	return ""
}

func ExpandTime(tp TimeParameters) (str string, err error) {
	var stime, sdate string
	daypos := Today
	if tp.LangCode == "" {
		log.Panic("ExpandTime, no language code")
	}
	storyTime, e := time.Parse(time.RFC3339, tp.TimeStr)
	if e != nil {
		err = errors.New("utime.ExpandTime: Error parsing time as RFC3339: " + e.Error())
		return tp.TimeStr, err
	}
	now := time.Now()
	elsewhere := false
	location, _ := time.LoadLocation(tp.ListenerTimeZone)
	if location != nil {
		t := storyTime.In(location)
		_, ol := t.Zone()
		n, ot := storyTime.Zone()
		//zlog.Info("stimezname:", n)
		if n != "UTC" && ol != ot {
			//zlog.Info("Listenerzone:", ol, "storyzone:", ot, tp.ListenerTimeZone, str)
			elsewhere = true
			now = now.In(location)
			storyTime = t
		}
	}
	if tp.ShowDate {
		daypos = GetRelativeDay(now, storyTime)
	}
	if tp.ShowTime {
		hour := storyTime.Hour()
		post := ""
		if !tp.Use24Hour {
			if hour >= 12 {
				post = " P.M."
			} else {
				post = " A.M."
			}
			if hour > 12 {
				hour -= 12
			}
		} else {
			if hour > 12 {
				hour -= 12
			}
		}
		mins := storyTime.Minute()
		if !tp.IsAccurate {
			mins = getRoundedMinutes(mins)
		}
		stime = fmt.Sprintf("%d:%02d", hour, mins) + post
		if stime == "0:00" && tp.LangCode == "en" {
			stime = "midnight"
		}
	}
	if tp.ShowDate {
		if daypos == Later || daypos == Earlier || tp.IsAccurate { // not today/tomorrow etc
			sy, sw := storyTime.ISOWeek()
			ny, nw := now.ISOWeek()
			if sy == ny && sw == nw { // in this week
				weekday := int(storyTime.Weekday())
				if stime != "" {
					sdate += weekdayNamePreposition[tp.LangCode]
				}
				sdate += " " + WeekdayName[LangValue{tp.LangCode, weekday}]
			} else {
				day := storyTime.Day()
				month := int(storyTime.Month())
				if sdate != "" {
					sdate += " "
				}
				sdate += fmt.Sprintf("%d %s", day, MonthName[LangValue{tp.LangCode, month}])
				if sy != ny { // this year
					sdate += fmt.Sprintf(" %d", storyTime.Year())
				}
			}
		} else {
			dayPeriod := getDayPeriodFromHour(storyTime.Hour())
			dayPosNameStr := dayPosName[LangValue{tp.LangCode, daypos}]
			if daypos == Yesterday && now.Hour() < 5 { // specifiy weekday as it's ambigous just after midnight
				weekday := int(storyTime.Weekday())
				dayPosNameStr = WeekdayName[LangValue{tp.LangCode, weekday}]
			}
			if tp.IsAccurate || !tp.Use24Hour || daypos != Today {
				if dayPeriod == Night && daypos == Tomorrow && tp.LangCode == "en" {
					sdate += " tonight"
				} else {
					sdate += " " + dayPosNameStr
				}
			} else {
				if daypos == Today {
					sdate += " " + todayPeriodExpression[LangValue{tp.LangCode, dayPeriod}]
				} else { // this will never happen
					format := daytimePeriodFormat[LangValue{tp.LangCode, dayPeriod}]
					sdate += " " + fmt.Sprintf(format, dayPosNameStr)
				}
			}
		}
	}
	prep := getPreposition(tp)
	str = zstr.Concat(" ", prep, stime, sdate)
	if tp.ShowTime && elsewhere {
		str += " " + localTimeExpression[tp.LangCode]
	}
	return
}
