package zlocale

import (
	"strings"

	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zwords"
)

//  Created by Tor Langballe on /1/11/15.

// see: https://github.com/RadhiFadlillah/sysloc

// TODO: Move out of zui?
var (
	IsMondayFirstInWeek          = zkeyvalue.NewOption[bool](nil, "ztime.IsMondayFirstInWeek", true)
	IsShowWeekNumbersInCalendars = zkeyvalue.NewOption[bool](nil, "ztime.IsShowWeekNumbersInCalendars", true)
	IsUse24HourClock             = zkeyvalue.NewOption[bool](nil, "ztime.IsUse24HourClock", true)
	IsShowMonthBeforeDay         = zkeyvalue.NewOption[bool](nil, "ztime.IsShowMonthBeforeDay", false)
	IsDisplayServerTime          = zkeyvalue.NewOption[bool](nil, "ztime.IsDisplayServerTime", true)
)

func GetDeviceLanguageCode() string {
	return "en"
}

func GetLangCodeAndCountryFromLocaleId(bcp string, forceNo bool) (string, string) { // lang, country-code
	lang, ccode := zstr.SplitInTwo(bcp, "-")
	if ccode == "" {
		_, ccode := zstr.SplitInTwo(bcp, "_")
		if ccode == "" {
			parts := strings.Split(bcp, "-")
			if len(parts) > 2 {
				return parts[0], parts[len(parts)-1]
			}
			return bcp, ""
		}
	}
	if lang == "nb" {
		lang = "no"
	}
	return lang, ccode
}

func UsesMetric() bool {
	return true
}

func UsesCelsius() bool {
	return true
}

func Uses24Hour() bool {
	return true
}

func FirstToTitleCaseExcept(str string, langCode string) (out string) {
	parts := strings.Split(str, " ")
	for i, p := range parts {
		if i != 0 && zwords.IsNonTitleableWord(p, langCode) {
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, " ")
}
