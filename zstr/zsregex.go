package zstr

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	InDoubleSquigglyBracketsRegex = regexp.MustCompile(`{{([^}]+)}}`)
	HashRegEx                     = regexp.MustCompile(`#([A-Za-z_]\w+)`) // (\s|\B) at start needed?
	EmailRegex                    = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	DollarArgRegex                = regexp.MustCompile(`\$(\w+)`)
)

// ReplaceAllCapturesFunc calls replace with contents of the first capture group and index 1, then next and index 2 etc.
// The returned string replaces the capture group, and the entire surrounding string and new contents is returned.
func ReplaceAllCapturesFunc(regex *regexp.Regexp, str string, replace func(cap string, index int) string) string {
	return replaceAllCaptures(regex, str, true, replace)
}

func ReplaceAllCapturesWithoutMatchFunc(regex *regexp.Regexp, str string, replace func(cap string, index int) string) string {
	return replaceAllCaptures(regex, str, false, replace)
}

func replaceAllCaptures(regex *regexp.Regexp, str string, keepMatch bool, replace func(cap string, index int) string) string {
	var out string
	groups := regex.FindAllStringSubmatchIndex(str, -1)
	if len(groups) == 0 {
		return str
	}
	var last int
	for _, group := range groups {
		glen := len(group)
		for i := 2; i < glen; i += 2 {
			s := group[i]
			e := group[i+1]
			if !keepMatch {
				ks := group[i-2]
				if s-ks > 0 {
					out += str[last:ks]
					last = s
				}
			}
			if s == -1 || e == -1 {
				// we don't set last, so this whole part is copied in next loop or end
				continue
			}
			out += str[last:s]
			last = e
			if !keepMatch {
				last = group[i-1]
			}
			out += replace(str[s:e], i/2)
		}
	}
	out += str[last:]
	return out
}

func GetAllCaptures(regex *regexp.Regexp, str string) []string {
	var out []string
	ReplaceAllCapturesFunc(regex, str, func(cap string, index int) string {
		out = append(out, cap)
		return ""
	})
	return out
}

func ReplaceHashTags(text string, f func(tag string) string) string {
	out := HashRegEx.ReplaceAllStringFunc(text, func(tag string) string {
		tag = strings.Replace(tag, "#", "", 1)
		return f(tag)
	})
	return out
}

func IsValidEmail(email string) bool {
	if len(email) < 3 && len(email) > 254 {
		return false
	}
	return EmailRegex.MatchString(email)
}

// WildcardAsteriskToRegExCapture turns *'s in a string to wildcard capture symbols
func WildcardAsteriskToRegExCapture(str string) (*regexp.Regexp, error) {
	str = regexp.QuoteMeta(str)
	str = strings.Replace(str, `\*`, "(.*)", -1)
	return regexp.Compile(str)
}

// A WildCardTransformer takes has a from: Big* and a to: Medium* which need the same amount of '*'s.
// Each * in from replaces the next * in to. BigDog -> MediumDog etc
// Big* to: *Medium would yield: BigDog -> DogMedium
type WildCardTransformer struct {
	regEx         *regexp.Regexp
	wildTo        string
	asteriskCount int
}

func NewWildCardTransformer(wildFrom, wildTo string) (*WildCardTransformer, error) {
	var err error

	cf := strings.Count(wildFrom, "*")
	ct := strings.Count(wildTo, "*")
	if cf != ct {
		return nil, fmt.Errorf("Mismatch in number of wildcard asterisks: %s != %s", cf, ct)
	}
	w := &WildCardTransformer{}
	w.asteriskCount = cf
	w.wildTo = wildTo
	w.regEx, err = WildcardAsteriskToRegExCapture(wildFrom)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, wildFrom)
	}
	return w, nil
}

func (w *WildCardTransformer) Transform(str string) (string, error) {
	matches := GetAllCaptures(w.regEx, str)
	if len(matches) != w.asteriskCount {
		return "", fmt.Errorf("input string has different nuber of matches than from wildcard wants: %d != %d", len(matches), w.asteriskCount)
	}
	replaced := w.wildTo
	for _, m := range matches {
		replaced = strings.Replace(replaced, "*", m, 1)
	}
	return replaced, nil
}
