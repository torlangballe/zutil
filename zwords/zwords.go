package zwords

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zstr"
)

const (
	KiloByte  = 1024
	MegaByte  = 1024 * 1024
	GigaByte  = 1024 * 1024 * 1024
	TerraByte = 1024 * 1024 * 1024 * 1024
)

// DefaultLanguage can be set with language gotten os/browser, if you use functions with langCode == "", this is used
var DefaultLanguage = "en"

// TS is a function to translate a string to current language
// Not implemented yet
var TS = func(str string) string {
	return str
}

// TSL uses TL but with a specific language to translate to
var TSL = func(str, langCode string) string {
	return str
}

func getSizeString(b int64, multiples int64, suffix, langCode string, maxSignificant int) string {
	prefs := []string{"", "K", "M", "G", "T", "P"}
	var n int64 = 1
	if maxSignificant == 0 {
		maxSignificant = 1
	}
	for _, pref := range prefs {
		if b < n*multiples {
			return NiceFloat(float64(b)/float64(n), maxSignificant) + " " + pref + suffix
		}
		n *= multiples
	}
	return "too big"
}

func GetBandwidthString(b int64, langCode string, maxSignificant int) string {
	return getSizeString(b, 1000, "b", langCode, maxSignificant)
}

func GetStorageSizeString(b int64, langCode string, maxSignificant int) string {
	return getSizeString(b, 1000, "B", langCode, maxSignificant)
}

func GetMemoryString(b int64, langCode string, maxSignificant int) string {
	return getSizeString(b, 1024, "B", langCode, maxSignificant)
}

// NiceFloat converts a float to string, with only significant amount of post-comma digits
func NiceFloat(f float64, significant int) string {
	var s string
	if significant == 0 {
		s = fmt.Sprintf("%f", f)
	} else {
		format := fmt.Sprintf("%%.%df", significant)
		s = fmt.Sprintf(format, f)
	}
	if strings.ContainsRune(s, '.') {
		for zstr.HasSuffix(s, "0", &s) {
		}
		zstr.HasSuffix(s, ".", &s)
	}
	return s
}

// PluralizeWord returns word if count == 1 or plural if != "".
// Otherwise it uses langauge-specific rules to pluralize.
// langCode == "" uses DefaultLanguage
func PluralizeWord(word string, count float64, langCode, plural string) string {
	if langCode == "" {
		langCode = DefaultLanguage
	}
	var str string
	if int64(count) != 1 {
		if plural != "" {
			str += plural
		} else {
			str += word
			switch langCode {
			case "", "en", "uk", "us":
				if strings.HasSuffix(word, "s") {
					str += "es"
				} else {
					str += "s"
				}
			case "no", "da", "sv":
				str += "er"
			}
		}
	} else {
		str += word
	}
	return str
}

// PluralizeWordWithTable uses PluralizeWord and a table of specific names for integers
func PluralizeWordWithTable(word string, count float64, langCode, plural string, table map[int]string) string {
	cw := table[int(count)]
	if cw == "" {
		cw = strconv.Itoa(int(count))
	}
	return cw + " " + PluralizeWord(word, count, langCode, plural)
}

// PluralWordWithCount uses PluralizeWordWithTable to create a nice count + pluralized word
func PluralWordWithCount(word string, count float64, langCode, plural string, significant int) string { // maybe just make the plural mandetory
	if langCode == "" {
		langCode = DefaultLanguage
	}
	scount := NiceFloat(count, significant)
	// zlog.Info("Plurlz:", scount, "'"+word+"'")
	return scount + " " + PluralizeWord(word, count, langCode, plural)
}

// Pluralize is a convenience function to pluralize words with int, default langage and only rule-based pluralization
func Pluralize(word string, count int) string {
	return PluralWordWithCount(word, float64(count), "", "", 0)
}

func Login() string {
	return TS("Log in") // generic name for login button etc
}

func Logout() string {
	return TS("Log out") // generic name for login button etc
}

func Register() string {
	return TS("Register") // generic name for register button (to register for a service)
}

func And() string {
	return TS("And") // generic name for and, i.e  cats and dogs
}

func Hour(count float32) string {
	if count != 1 {
		return TS("hours") // generic name for hours plural
	}
	return TS("hour") // generic name for hour singular
}

func Today() string {
	return TS("Today") // generic name for today
}

func Yesterday() string {
	return TS("Yesterday") // generic name for yesterday
}

func Tomorrow() string {
	return TS("Tomorrow") // generic name for tomorrow
}

func Minute(count float32) string {
	if count != 1 {
		return TS("minutes") // generic name for minutes plural
	}
	return TS("minute") // generic name for minute singular
}

func Meter(count float32, langCode string) string {
	if count != 1 {
		return TSL("meters", langCode) // generic name for meters plural
	}
	return TSL("meter", langCode) // generic name for meter singular
}

func KiloMeter(count float32, langCode string) string {
	if count != 1 {
		return TSL("kilometers", langCode) // generic name for kilometers plural
	}
	return TSL("kilometer", langCode) // generic name for kilometer singular
}

func Mile(count float32, langCode string) string {
	if count != 1 {
		return TSL("miles", langCode) // generic name for miles plural
	}
	return TSL("mile", langCode) // generic name for mile singular
}

func Yard(count float32, langCode string) string {
	if count != 1 {
		return TSL("yards", langCode) // generic name for yards plural
	}
	return TSL("yard", langCode) // generic name for yard singular
}

func Inch(count float32, langCode string) string {
	if count != 1 {
		return TSL("inches", langCode) // generic name for inch plural
	}
	return TSL("inch", langCode) // generic name for inches singular
}

func DayPeriod() string     { return TS("am/pm") }      // generic name for am/pm part of day when used as a column title etc
func OK() string            { return TS("OK") }         // generic name for OK in button etc
func Set() string           { return TS("Set") }        // generic name for Set in button, i.e set value
func Clear() string         { return TS("Clear") }      // generic name for Set in button, i.e set value
func Off() string           { return TS("Off") }        // generic name for Off in button, i.e value/switch is off. this is RMEOVED by VO in value
func Open() string          { return TS("Open") }       // generic name for button to open a window or something
func Back() string          { return TS("Back") }       // generic name for back button in navigation bar
func Cancel() string        { return TS("Cancel") }     // generic name for Cancel in button etc
func Close() string         { return TS("Close") }      // generic name for Close in button etc
func Play() string          { return TS("Play") }       // generic name for Play in button etc
func Post() string          { return TS("Post") }       // generic name for Post in button etc, post a message to social media etc
func Edit() string          { return TS("Edit") }       // generic name for Edit in button etc, to start an edit action
func Reset() string         { return TS("Reset") }      // generic name for Reset in button etc, to reset/restart something
func Pause() string         { return TS("Pause") }      // generic name for Pause in button etc
func Save() string          { return TS("Save") }       // generic name for Save in button etc
func Add() string           { return TS("Add") }        // generic name for Add in button etc
func Delete() string        { return TS("Delete") }     // generic name for Delete in button etc
func Exit() string          { return TS("Exit") }       // generic name for Exit in button etc. i.e  You have unsaved changes. [Save] [Exit]
func RetryQuestion() string { return TS("Retry?") }     // generic name for Retry? in button etc, must be formulated like a question
func Fahrenheit() string    { return TS("Fahrenheit") } // generic name for fahrenheit, used in buttons etc.
func Celsius() string       { return TS("Celsius") }    // generic name for celsius, used in buttons etc.
func Settings() string      { return TS("Settings") }   // generic name for settings, used in buttons / title etc
func DayOfMonth() string    { return TS("Day") }        // generic name for the day of a month i.e 23rd of July
func Month() string         { return TS("Month") }      // generic name for month.
func Year() string          { return TS("Year") }       // generic name for year.

func SetOrClear(set bool) string {
	if set {
		return Set()
	}
	return Clear()
}

func Is(count float64) string {
	if count == 1 {
		return TS("is") // there IS a dog at school
	}
	return TS("are") // there ARE 5 dogs at school
}

func Day(count float32) string {
	if count != 1 {
		return TS("Days") // generic name for the plural of a number of days since/until etc
	}
	return TS("Day") // generic name for a days since/until etc
}

func Selected(on bool) string {
	if on {
		return TS("Selected") // generic name for selected in button/title/switch, i.e something is selected/on
	} else {
		return TS("Unselected") // generic name for unselected in button/title/switch, i.e something is unselected/off
	}
}

func MonthFromNumber(m, chars int) string {
	var str = ""
	switch m {
	case 1:
		str = TS("January") // name of month
	case 2:
		str = TS("February") // name of month
	case 3:
		str = TS("March") // name of month
	case 4:
		str = TS("April") // name of month
	case 5:
		str = TS("May") // name of month
	case 6:
		str = TS("June") // name of month
	case 7:
		str = TS("July") // name of month
	case 8:
		str = TS("August") // name of month
	case 9:
		str = TS("September") // name of month
	case 10:
		str = TS("October") // name of month
	case 11:
		str = TS("November") // name of month
	case 12:
		str = TS("December") // name of month
	default:
		break
	}
	if chars != -1 {
		str = zstr.Head(str, chars)
	}
	return str
} // generic name for year.

func NameOfLanguageCode(langCode, inLanguage string) string {
	if inLanguage == "" {
		inLanguage = "en"
	}
	switch strings.ToLower(langCode) {
	case "en":
		return TS("English") // name of english language
	case "de":
		return TS("German") // name of german language
	case "ja", "jp":
		return TS("Japanese") // name of english language
	case "no", "nb", "nn":
		return TS("Norwegian") // name of norwegian language
	case "us":
		return TS("American") // name of american language/person
	case "ca":
		return TS("Canadian") // name of canadian language/person
	case "nz":
		return TS("New Zealander") // name of canadian language/person
	case "at":
		return TS("Austrian") // name of austrian language/person
	case "ch":
		return TS("Swiss") // name of swiss language/person
	case "in":
		return TS("Indian") // name of indian language/person
	case "gb", "uk":
		return TS("British") // name of british language/person
	case "za":
		return TS("South African") // name of south african language/person
	case "ae":
		return TS("United Arab Emirati") // name of UAE language/person
	case "id":
		return TS("Indonesian") // name of indonesian language/person
	case "sa":
		return TS("Saudi Arabian") // name of saudi language/person
	case "au":
		return TS("Australian") // name of australian language/person
	case "ph":
		return TS("Filipino") // name of filipino language/person
	case "sg":
		return TS("Singaporean") // name of singaporean language/person
	case "ie":
		return TS("Irish") // name of irish language/person
	default:
		return ""
	}
}

func WordsForOneDigitNumber(n int, lang string) string {
	switch lang {
	case "en", "":
		switch n {
		case 0:
			return "zero"
		case 1:
			return "one"
		case 2:
			return "two"
		case 3:
			return "three"
		case 4:
			return "four"
		case 5:
			return "five"
		case 6:
			return "six"
		case 7:
			return "seven"
		case 8:
			return "eight"
		case 9:
			return "nine"
		}
	}
	return strconv.Itoa(n)
}

func Distance(meters float64, metric bool, langCode string, round bool) string {
	const (
		meter = iota + 1
		km
		mile
		yard
	)

	var dtype = meter
	var d = meters
	var distance = ""
	var word = ""

	if metric {
		if d >= 1000 {
			dtype = km
			d /= 1000
		}
	} else {
		d /= 1.0936133
		if d >= 1760 {
			dtype = mile
			d /= 1760
			distance = fmt.Sprintf("%.1f", d)
		} else {
			dtype = yard
			d = math.Floor(d)
			distance = strconv.Itoa(int(d))
		}
	}
	switch dtype {
	case meter:
		word = Meter(2, langCode)

	case km:
		word = KiloMeter(2, langCode)

	case mile:
		word = Mile(2, langCode)

	case yard:
		word = Yard(2, langCode)
	}
	if dtype == meter || dtype == yard && round {
		d = math.Ceil(((math.Ceil(d) + 9) / 10) * 10)
		distance = strconv.Itoa(int(d))
	} else if round && d > 50 {
		distance = fmt.Sprintf("%d", int(d))
	} else {
		distance = fmt.Sprintf("%.1f", d)
	}
	return distance + " " + word
}

/*
func WordsMemorySizeAsstring(b int64, langCode string, maxSignificant int, isBits bool) string {
	kiloByte := 1024.0
	megaByte := kiloByte * 1024
	gigaByte := megaByte * 1024
	terraByte := gigaByte * 1024
	var word = "T"
	var n = float64(b) / terraByte
	d := float64(b)
	if d < kiloByte {
		word = ""
		n = float64(b)
	} else if d < megaByte {
		word = "K"
		n = float64(b) / kiloByte
	} else if d < gigaByte {
		word = "M"
		n = float64(b) / megaByte
	} else if d < terraByte {
		word = "G"
		n = float64(b) / gigaByte
	}
	if isBits {
		word += "b"
	} else {
		word += "B"
	}
	str := NiceFloat(n, maxSignificant) + " " + word
	return str
}
*/

func IsNonTitleableWord(word, langCode string) bool {
	switch langCode {
	case "en", "":
		switch strings.ToLower(word) {
		case "this", "a", "the", "an", "and", "of":
			return true
		}
	}
	return false
}
