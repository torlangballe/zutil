package zstr

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	InDoubleSquigglyBracketsRegex = regexp.MustCompile(`{{([^}]+)}}`)
	InSingleSquigglyBracketsRegex = regexp.MustCompile(`{([^}]+)}`)
	HashRegEx                     = regexp.MustCompile(`#([A-Za-z_]\w+)`) // (\s|\B) at start needed?
	EmailRegex                    = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	DollarArgRegex                = regexp.MustCompile(`\$(\w+)`)
)

type RegOpts int

const (
	RegWithoutMatch RegOpts = 1 << iota // RegWithoutMatch means only include what's in ( ) in the capture string
	RegWithOutside
)

// ReplaceAllCapturesFunc calls replace with contents of the first capture group and index 1, then next and index 2 etc.
// The returned string replaces the capture group, and the entire surrounding string and new contents is returned.
func ReplaceAllCapturesFunc(regex *regexp.Regexp, str string, opts RegOpts, replace func(cap string, index int) string) string {
	var out string
	var outIndex int = -1
	// fmt.Println("ReplaceAllCapturesFunc:", str)
	groups := regex.FindAllStringSubmatchIndex(str, -1)
	if len(groups) == 0 {
		addOutput(&out, str, &outIndex, opts, replace)
		return out
	}
	var last int
	for _, group := range groups {
		glen := len(group)
		for i := 2; i < glen; i += 2 {
			s := group[i]
			e := group[i+1]
			if opts&RegWithoutMatch != 0 {
				ks := group[i-2]
				if s-ks > 0 {
					addOutput(&out, str[last:ks], &outIndex, opts, replace)
					last = s
				}
			}
			if s == -1 || e == -1 {
				// we don't set last, so this whole part is copied in next loop or end
				continue
			}
			addOutput(&out, str[last:s], &outIndex, opts, replace)
			last = e
			if opts&RegWithoutMatch != 0 {
				last = group[i-1]
			}
			out += replace(str[s:e], i/2)
		}
	}
	addOutput(&out, str[last:], &outIndex, opts, replace)
	// fmt.Println("ReplaceAllCapturesFunc2:", out)
	return out
}

func addOutput(out *string, str string, outIndex *int, opts RegOpts, replace func(cap string, index int) string) {
	if opts&RegWithOutside != 0 {
		str = replace(str, *outIndex)
		(*outIndex)--
	}
	*out += str
}

func GetAllCaptures(regex *regexp.Regexp, str string) []string {
	var out []string
	ReplaceAllCapturesFunc(regex, str, 0, func(cap string, index int) string {
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
	regStr := regexp.QuoteMeta(str)
	regStr = strings.Replace(regStr, `\*`, "(.*)", -1)
	// fmt.Println("WildcardAsteriskToRegExCapture:", str, "->", regStr)
	return regexp.Compile(regStr)
}

// A WildCardTransformer takes has a from: Big* and a to: Medium* which need the same amount of '*'s.
// Each * in wildFrom replaces the next * in wildTo.
// Big* to: *Medium would yield: BigDog -> DogMedium
type WildCardTransformer struct {
	regEx         *regexp.Regexp
	wildTo        string
	NoOp          bool
	asteriskCount int
}

func NewWildCardTransformer(wildFrom, wildTo string) (*WildCardTransformer, error) {
	w := &WildCardTransformer{}
	if wildFrom == "" || wildTo == "" {
		w.NoOp = true
		return w, nil
	}
	var err error
	cf := strings.Count(wildFrom, "*")
	ct := strings.Count(wildTo, "*")
	if cf < ct {
		return nil, fmt.Errorf("Mismatch in number of wildcard asterisks: %d vs %d", cf, ct)
	}
	w.asteriskCount = cf
	w.wildTo = wildTo
	w.regEx, err = WildcardAsteriskToRegExCapture(wildFrom)
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, wildFrom)
	}
	return w, nil
}

func (w *WildCardTransformer) Transform(str string) (string, error) {
	if w.NoOp {
		return str, nil
	}
	matches := GetAllCaptures(w.regEx, str)
	// fmt.Printf("WildCardTransformer: %s %v %+v\n", str, w.regEx.Expand, matches)

	if len(matches) != w.asteriskCount {
		return "", fmt.Errorf("input string has different number of matches than from wildcard wants: %d != %d (%s)", len(matches), w.asteriskCount, str)
	}
	replaced := w.wildTo
	for _, m := range matches {
		replaced = strings.Replace(replaced, "*", m, 1)
	}
	return replaced, nil
}

func ReplaceInSquigglyBrackets(str string, replace func(string) string) string {
	out := ReplaceAllCapturesFunc(InSingleSquigglyBracketsRegex, str, RegWithoutMatch, func(s string, index int) string {
		return replace(s)
	})
	return out
}
