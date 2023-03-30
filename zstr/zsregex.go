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
	fmt.Println("Groups:", groups, str)
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
		fmt.Println("AllCaps:", cap)
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
