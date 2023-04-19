package zlocale

import (
	"strings"

	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zstr"
)

//  Created by Tor Langballe on /1/11/15.

// https://github.com/RadhiFadlillah/sysloc

// TODO: Move out of zui?
var (
	IsMondayFirstInWeek       = Option[bool]{Key: "ztime.IsMondayFirstInWeek", Default: true}
	IsShowWeekdaysInCalendars = Option[bool]{Key: "ztime.IsShowWeekdaysInCalendars", Default: true}
	IsUse24HourClock          = Option[bool]{Key: "ztime.IsUse24HourClock", Default: true}
)

type Option[V any] struct {
	Key         string
	Default     V
	MakeDefault func() V
	value       V
	read        bool
}

func (t *Option[V]) Get() V {
	if !t.read {
		if !zkeyvalue.DefaultStore.GetItem(t.Key, &t.value) {
			if t.MakeDefault != nil {
				t.value = t.MakeDefault()
			} else {
				t.value = t.Default
			}
		}
		t.read = true
	}
	return t.value
}

func (t *Option[V]) Set(v V) {
	zkeyvalue.DefaultStore.SetItem(t.Key, t.value, true)
}

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
