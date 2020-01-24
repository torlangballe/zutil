package zwords

import (
	"fmt"
	"strings"

	"github.com/torlangballe/zutil/zstr"
)

const (
	KKiloByte  = 1024
	KMegaByte  = 1024 * 1024
	KGigaByte  = 1024 * 1024 * 1024
	KTerraByte = 1024 * 1024 * 1024 * 1024
)

func GetBandwidthString(b int64, langCode string, maxSignificant int) string {
	s := "Bit"
	v := 0.0
	switch {
	case b < 1000:
		v = float64(b)
	case b < 1000*1000:
		s = "K" + s
		v = float64(b) / float64(1000)
	case b < 1000*1000*1000:
		s = "M" + s
		v = float64(b) / float64(1000*1000)
	default:
		s = "T" + s
		v = float64(b) / float64(1000*1000*1000)
	}
	return PluralWord(s, v, langCode, "", maxSignificant)
}

func GetMemoryString(b int64, langCode string, maxSignificant int) string {
	s := "Byte"
	v := 0.0
	switch {
	case b < KKiloByte:
		v = float64(b)
	case b < KMegaByte:
		s = "K" + s
		v = float64(b) / float64(KKiloByte)
	case b < KGigaByte:
		s = "M" + s
		v = float64(b) / float64(KMegaByte)
	default:
		s = "T" + s
		v = float64(b) / float64(KGigaByte)
	}
	return PluralWord(s, v, langCode, "", maxSignificant)
}

func NiceFloat(f float64, significant int) string {
	format := fmt.Sprintf("%%.%df", significant)
	s := fmt.Sprintf(format, f)
	if strings.ContainsRune(s, '.') {
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, "0", &s)
		zstr.HasSuffix(s, ".", &s)
	}
	return s
}

func PluralWord(word string, count float64, langCode, plural string, significant int) string { // maybe just make the plural mandetory
	str := NiceFloat(count, significant) + " "
	if int64(count) != 1 {
		if plural != "" {
			str += plural
		} else {
			str += word
			switch langCode {
			case "en", "uk", "us":
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
