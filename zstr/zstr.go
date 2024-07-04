package zstr

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	uuidv4 "github.com/bitactro/UUIDv4"
	"github.com/torlangballe/zutil/zint"
)

const Digits = "0123456789"

type StrIDer interface {
	GetStrID() string
}

type CreateStrIDer interface {
	CreateStrID()
}

type TitleOwner interface {
	GetTitle() string
}

type NameGetter interface {
	GetName() string
}

type KeyValue struct {
	Key   string
	Value string
}

type StrInt struct {
	Str string
	Int int64
}

func GetLevenshteinRatio(a, b string) float64 { // returns distance / min length of a or b
	len := float64(zint.Min(len(a), len(b)))
	return float64(GetLevenshteinDistance(a, b)) / len
}

func GetLevenshteinDistance(a, b string) int {
	f := make([]int, utf8.RuneCountInString(b)+1)

	for j := range f {
		f[j] = j
	} // for i

	for _, ca := range a {
		j := 1
		fj1 := f[0] // fj1 is the value of f[j - 1] in last iteration
		f[0]++
		for _, cb := range b {
			mn := zint.Min(f[j]+1, f[j-1]+1) // delete & insert
			if cb != ca {
				mn = zint.Min(mn, fj1+1) // change
			} else {
				mn = zint.Min(mn, fj1) // matched
			} // else

			fj1, f[j] = f[j], mn // save f[j] to fj1(j is about to increase), update f[j] to mn
			j++
		} // for cb
	} // for ca

	return f[len(f)-1]
}

func Head(str string, length int) string {
	return fmt.Sprintf("%.*s", length, str)
}

func Body(str string, pos, length int) string {
	rs := []rune(str)
	rl := len(rs)
	if pos < 0 {
		pos = 0
	}
	if pos >= rl {
		return ""
	}
	if length == -1 {
		length = rl - pos
	}
	e := pos + length
	if e > rl {
		e = rl
	}
	if e-pos == 0 {
		return ""
	}
	return string(rs[pos:e])
}

func Map(str string, convert func(i int, r rune) string) string {
	var out string
	for i, r := range str {
		out += convert(i, r)
	}
	return out
}

func HeadUntilCharSet(str, chars string) string {
	i := strings.IndexAny(str, chars)
	if i == -1 {
		return str
	}
	return str[:i]
}

func HeadUntil(str, sep string) string {
	i := strings.Index(str, sep)
	if i == -1 {
		return str
	}
	return str[:i]
}

func HeadUntilLast(str, sep string, rest *string) string {
	i := strings.LastIndex(str, sep)
	if i == -1 {
		return str
	}
	if rest != nil {
		*rest = str[i+1:]
	}
	return str[:i]
}

func HeadUntilIncluding(str, sep string, rest *string) string {
	i := strings.Index(str, sep)
	if i == -1 {
		return str
	}
	if rest != nil {
		*rest = str[i+len(sep):]
	}
	return str[:i] + sep
}

func HeadUntilWithRest(str, sep string, rest *string) string {
	i := strings.Index(str, sep)
	if i == -1 {
		return str
	}
	*rest = str[i+len(sep):]
	return str[:i]
}

func Tail(str string, length int) string {
	l := zint.Clamp(length, 0, len(str)) // hack without unicode support
	return str[len(str)-l:]
}

func TailUntil(str, sep string) string {
	i := strings.LastIndex(str, sep)
	if i == -1 {
		return str
	}
	return str[i+len(sep):]
}

func TailUntilWithRest(str, sep string, rest *string) string {
	i := strings.LastIndex(str, sep)
	if i == -1 {
		return str
	}
	*rest = str[:i]
	return str[i+len(sep):]
}

func TruncatedCharsAtEnd(str string, chars int) (s string) {
	if str != "" {
		r := []rune(str)
		l := len(r)
		if chars < l {
			str = string(r[:l-chars])
		}
	}
	return str
}

func TruncatedFromEnd(str string, length int, endString string) (s string) {
	if str != "" {
		r := []rune(str)
		l := len(r)
		if l > length {
			str = string(r[:length])
			str += endString
		}
	}
	return str
}

func TruncatedMiddle(str string, length int, moreString string) (s string) {
	if str != "" {
		r := []rune(str)
		l := len(r)
		if l > length {
			m := length / 2
			str = string(r[:m]) + moreString + string(r[l-m:])
		}
	}
	return str
}

func TruncatedAtCharFromEnd(str string, max int, divider, truncateSymbol string) (s string) {
	if utf8.RuneCountInString(str) <= max {
		return str
	}
	out := ""
	parts := strings.Split(str, divider)
	len := 0
	for i, p := range parts {
		if i != 0 {
			out += divider
			len++
		}
		out += p
		len += utf8.RuneCountInString(p)
		if len >= max {
			return out + truncateSymbol
		}
	}
	return out
}

func After(str, after string) string {
	i := strings.Index(str, after)
	if i == -1 {
		return ""
	}
	return str[i+len(after):]
}

func TruncatedFromStart(str string, length int, endString string) string {
	slen := len(str)
	if slen <= length {
		return str
	}
	return endString + str[slen-length:]
}

// Concatinates parts, adding divider if prev or current added is not empty
// Doesn't add divider if prev ends in divider og next part begins with it
func Concat(divider string, parts ...any) string {
	var str string
	for _, p := range parts {
		s := fmt.Sprintf("%v", p)
		if s != "" {
			if str == "" {
				str = s
			} else {
				prevHas := strings.HasSuffix(str, divider)
				currentHas := strings.HasPrefix(s, divider)
				if !prevHas && !currentHas {
					str += divider
				}
				if prevHas && currentHas {
					str = TruncatedCharsAtEnd(str, 1)
				}
				str += s
			}
		}
	}
	return str
}

func Spaced(parts ...any) string {
	return Concat(" ", parts...)
}

func AnySliceToStrings(parts []any) []string {
	s := make([]string, len(parts))
	for i, p := range parts {
		s[i] = fmt.Sprint(p)
	}
	return s
}

func StringsToAnySlice(parts []string) []any {
	a := make([]any, len(parts))
	for i, p := range parts {
		a[i] = p
	}
	return a
}

func IndexOf(str string, strs []string) int {
	for i, s := range strs {
		//		fmt.Print("IndexOf: '", str, "' : '", s, "'\n")
		if s == str {
			return i
		}
	}
	return -1
}

func StringsContain(list []string, str string) bool {
	for _, s := range list {
		if s == str {
			return true
		}
	}
	return false
}

func ContainsDuplicates(strs []string) bool {
	var m = map[string]bool{}

	for _, s := range strs {
		if m[s] {
			return true
		}
		m[s] = true
	}
	return false
}

func RemoveDuplicates(strs []string) (out []string) {
	m := map[string]bool{}
	for _, s := range strs {
		if !m[s] {
			out = append(out, s)
			m[s] = true
		}
	}
	return
}

func ExtractStringTilSeparator(str *string, sep string) (got string) {
	i := strings.Index(*str, sep)
	if i == -1 {
		got = *str
		*str = ""
		return
	}
	got = (*str)[:i]
	*str = (*str)[i+len(sep):]
	return
}

func ExtractStringFromEndTilSeparator(str *string, sep string) (got string) {
	i := strings.LastIndex(*str, sep)
	switch i {
	case -1:
		return
	default:
		got = (*str)[i+len(sep):]
		*str = (*str)[:i]
		return
	}
}

func ExtractItemFromStrings(strs *[]string, item string) bool {
	i := IndexOf(item, *strs)
	if i == -1 {
		return false
	}
	*strs = append((*strs)[:i], (*strs)[i+1:]...)
	return true
}

// ExtractFlaggedArg extracts the value after a flag string, removing them from strs
// i.e command -flag value : ExtractFlaggedArg(&args, "-flag", &value)
func ExtractFlaggedArg(strs *[]string, flag string, value *string) bool {
	i := IndexOf(flag, *strs)
	if i == -1 {
		return false
	}
	if len(*strs) < i+2 {
		return false
	}
	*value = (*strs)[i+1]
	*strs = append((*strs)[:i], (*strs)[i+2:]...)
	return true
}

func ExtractFirstString(strs *[]string) string {
	if len(*strs) == 0 {
		return ""
	}
	s := (*strs)[0]
	*strs = (*strs)[1:]

	return s
}

func ExtractLastString(strs *[]string) string {
	l := len(*strs)
	if l == 0 {
		return ""
	}
	s := (*strs)[l-1]
	*strs = (*strs)[:l-1]

	return s
}

// RemovedFromSlice returns the slice with the first instance of a string == str removed
func RemovedFromSlice(base []string, str string) (result []string) {
	i := IndexOf(str, base)
	if i == -1 {
		return base
	}
	return append(base[:i], base[i+1:]...)
}

func SliceToLower(slice []string) (out []string) {
	out = make([]string, len(slice))
	for i, s := range slice {
		out[i] = strings.ToLower(s)
	}
	return
}

func RemoveStringSlices(base, sub []string) (result []string) {
	for _, s := range base {
		if IndexOf(s, sub) == -1 {
			result = append(result, s)
		}
	}
	return
}

func UnionStringSet(aset, bset []string) []string {
	for _, b := range bset {
		if IndexOf(b, aset) == -1 {
			aset = append(aset, b)
		}
	}
	return aset
}

func SlicesIntersect(aset, bset []string) bool {
	for _, b := range bset {
		if IndexOf(b, aset) != -1 {
			return true
		}
	}
	return false
}

func SlicesAreEqual(aset, bset []string) bool {
	if len(aset) != len(bset) {
		return false
	}
	for _, b := range bset {
		if IndexOf(b, aset) == -1 {
			return false
		}
	}
	return true
}

func AddToSet(strs *[]string, str ...string) int {
	var count int
	for _, s := range str {
		if StringsContain(*strs, s) {
			continue
		}
		*strs = append(*strs, s)
		count++
	}
	return count
}

func GenerateRandomBytes(count int) []byte {
	data := make([]byte, count)
	n, err := io.ReadFull(crand.Reader, data)
	if n != len(data) || err != nil {
		panic(err)
	}
	return data
}

func GenerateRandomHexBytes(byteCount int) string {
	data := GenerateRandomBytes(byteCount)
	return hex.EncodeToString(data)
}

func GenerateUUID() string {
	return uuidv4.GenerateUUID4()
}

// IsRuneASCIIPrintable returns true if b is ascii and not control-character (space returns true)
func IsRuneASCIIPrintable(b rune) bool {
	return b >= ' ' && b <= '~'
}

func IsRuneASCIIAlpha(b rune) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

func IsRuneASCIINumeric(b rune) bool {
	return b >= '0' && b <= '9'
}

func IsRuneASCIIAlphaNumeric(b rune) bool {
	return IsRuneASCIIAlpha(b) || IsRuneASCIINumeric(b)
}

func IsRuneHex(b rune) bool {
	return IsRuneASCIINumeric(b) || b >= 'A' && b <= 'F' || b >= 'a' && b <= 'f'
}

func IsRuneValidInUUID(b rune) bool {
	return IsRuneHex(b) || b == '-'
}

func MD5Hex(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}

func SHA256Hex(data []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(data))
}

func CountWords(str string) int {
	return len(strings.Fields(str)) // we can make this better at some point
}

func HasPrefix(str, prefix string, rest *string) bool {
	if prefix == "" {
		*rest = str
		return true
	}
	if strings.HasPrefix(str, prefix) {
		*rest = str[len(prefix):]
		return true
	}
	return false
}

func HasSuffix(str, suffix string, rest *string) bool {
	if strings.HasSuffix(str, suffix) {
		*rest = str[:len(str)-len(suffix)]
		return true
	}
	return false
}

func TrimCommonExtremity(slice []string, head bool) string {
	stub := SliceCommonExtremity(slice, head)
	//	fmt.Println("TrimCommonExtremity:", stub, head)
	if stub != "" {
		for i, s := range slice {
			if head {
				slice[i] = strings.TrimPrefix(s, stub)
			} else {
				slice[i] = strings.TrimSuffix(s, stub)
			}
		}
	}
	return stub
}

func SliceCommonExtremity(slice []string, head bool) string {
	length := len(slice)
	if length == 0 {
		return ""
	}
	if length == 1 {
		return slice[0]
	}
	chars := 1
	var old, first string
	for {
		for i, s := range slice {
			var h string
			if head {
				h = Head(s, chars)
			} else {
				h = Tail(s, chars)
			}
			if i == 0 {
				first = h
				if first == old { // it's full size
					return first
				}
			} else {
				if h != first {
					return old
				}
			}
		}
		old = first
		chars++
	}
}

func HasPrefixNoCase(str, prefix string, rest *string) bool {
	return HasPrefix(strings.ToLower(str), strings.ToLower(prefix), rest)
}

func HasSuffixNoCase(str, suffix string, rest *string) bool {
	return HasSuffix(strings.ToLower(str), strings.ToLower(suffix), rest)
}

func GetQuotedArgs(args string) (parts []string) {
	RangeQuotedText(args, `"`, func(s string, inQuote bool) {
		if inQuote {
			parts = append(parts, s)
		} else if s != "" {
			s = strings.TrimSpace(s)
			if s != "" {
				split := strings.Fields(strings.TrimSpace(s))
				parts = append(parts, split...)
			}
		}
	})
	return
}

func FormatJSON(input []byte) (out string, err error) {
	buf := bytes.NewBuffer([]byte{})
	err = json.Indent(buf, input, "", "\t")
	if err != nil {
		return
	}
	out = buf.String()
	return
}

func SplitN(str, sep string, parts ...*string) bool {
	slice := strings.SplitN(str, sep, len(parts))
	if len(slice) == len(parts) {
		for i, s := range slice {
			if parts[i] != nil {
				*parts[i] = s
			}
		}
		return true
	}
	return false
}

func SplitByAnyOf(str string, seps []string, skipEmpty bool) []string {
	parts := []string{str}
	for _, sep := range seps {
		var splits []string
		for _, p := range parts {
			split := strings.Split(p, sep)
			splits = append(splits, split...)
		}
		parts = splits
	}
	// fmt.Println("SplitByAnyOf:", seps, "str:", str, "parts:", parts)
	if skipEmpty {
		for i := 0; i < len(parts); {
			if parts[i] == "" {
				parts = append(parts[:i], parts[i+1:]...)
			} else {
				i++
			}
		}
	}
	return parts
}

func AddMapToMap(m *map[string]string, add map[string]string) {
	for k, v := range add {
		(*m)[k] = v
	}
}

func CopyMap(m map[string]string) (n map[string]string) {
	n = make(map[string]string, len(m))
	for k, v := range m {
		n[k] = v
	}
	return
}

func CopyToEmpty(dest *string, str string) {
	if *dest == "" {
		*dest = str
	}
}

// SlicesIdentical compares if slices have same contents in same order
func SlicesIdentical(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// SlicesHaveSameValues compares if slices have same contents in any order
func SlicesHaveSameValues(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	sa := CopySlice(a)
	sort.Strings(sa)
	sb := CopySlice(b)
	sort.Strings(sb)
	return SlicesIdentical(sa, sb)
}

func CopySlice(s []string) []string {
	n := make([]string, len(s), len(s))
	copy(n, s)
	return n
}

func ReplaceVariablesWithValues(text, prefix string, values map[string]string) (content string) {
	spairs := make([]string, len(values)*2+2)
	keys := make([]string, 0, len(values))
	j := 0
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys) // sorts the keys

	for i := len(keys) - 1; i >= 0; i-- { // gets them in reverse, so StoryStartHour before StoryStart
		spairs[j] = prefix + keys[i]
		spairs[j+1] = values[keys[i]]
		j += 2
	}
	spairs[j] = prefix + prefix
	spairs[j+1] = prefix
	j += 2
	replacer := strings.NewReplacer(spairs...)

	const maxIter = 5 // maximum number of iterations
	var iter int
	var oldContent string
	content = text
	for iter < maxIter {
		content = replacer.Replace(content)
		if content == oldContent {
			break
		}
		oldContent = content
		iter++
	}
	return
}

func ReplaceWithFunc(str string, replace func(rune) string) string {
	var out string
	for _, r := range str {
		out += replace(r)
	}
	return out
}

func CreateFilterFunc(keep func(r rune) bool) func(string) string {
	return func(s string) string {
		return FilterWithFunc(s, keep)
	}
}

func CreateMultiFilterFunc(funcs []func(r rune) bool) func(string) string {
	return func(s string) string {
		for _, fn := range funcs {
			s = FilterWithFunc(s, fn)
		}
		return s
	}
}

func FilterWithFunc(str string, keep func(r rune) bool) string {
	var out string

	for _, r := range str {
		if keep(r) {
			out += string(r)
		}
	}
	return out
}

func Replace(str *string, find, with string) bool {
	text := strings.Replace(*str, find, with, -1)
	if *str != text {
		*str = text
		return true
	}
	return false
}

// func FilterCharsFromEditor(r rune) rune { ???
// 	switch {
// 	case r == 0xA0:
// 		return ' '
// 	}
// 	return r
// }

func ToASCIICode(r rune) rune {
	switch {
	case r <= 127:
		return r
	case r == 0xA0:
		return ' '
	}
	return '•'
}

func SplitByNewLines(str string, skipEmpty bool) []string {
	return SplitByAnyOf(str, []string{"\r\n", "\n", "\r"}, skipEmpty)
}

// TODO: remove, we can do range ourself
func RangeStringLines(str string, skipEmpty bool, f func(s string) bool) {
	for _, l := range SplitByNewLines(str, skipEmpty) {
		if !f(l) {
			break
		}
	}
}

// func GetStringSliceFromIntBase10(ids []int64) []string {
// 	s := make([]string, len(ids))
// 	for i := range ids {
// 		s[i] = strconv.FormatInt(ids[i], 10)
// 	}
// 	return s
// }

func FirstToTitleCase(str string) (out string) {
	if str == "" {
		return ""
	}
	r := []rune(str)
	r[0] = unicode.ToTitle(r[0])
	out = string(r)
	return
}

func FirstToLower(str string) (out string) {
	r := []rune(str)
	r[0] = unicode.ToLower(r[0])
	out = string(r)
	return
}

func FirstToLowerWithAcronyms(str string) (out string) {
	firstLower := -1
	for i, c := range str {
		if unicode.ToLower(c) == c {
			firstLower = i
			break
		}
	}
	if firstLower <= 0 {
		return strings.ToLower(str)
	}
	if firstLower == 1 {
		return FirstToLower(str)
	}
	out = strings.ToLower(str[:firstLower-1])
	out += str[firstLower-1:]
	return
}

func IsFirstLetterLowerCase(str string) bool {
	r := FirstRune(str)
	return unicode.ToLower(r) == r
}

func IsFirstLetterUpperCase(str string) bool {
	r := FirstRune(str)
	return unicode.ToUpper(r) == r
}

func FirstRune(str string) rune {
	r, _ := utf8.DecodeRuneInString(str)
	return r
}

func GetUntilChars(str, chars string) string {
	i := strings.IndexAny(str, chars)
	if i == -1 {
		return str
	}
	return str[:i]
}

func RangeQuotedText(str, quoteChar string, f func(s string, inQuote bool)) {
	parts := strings.Split(str, quoteChar)
	for i, p := range parts {
		f(p, i%2 == 1)
	}
}

func RangeInFromToSymbolsInText(str, start, end string, f func(s string, in bool)) {
	var index, old int
	for {
		if IndexFrom(str, start, &index) {
			index++
			s := index
			if IndexFrom(str, end, &index) {
				e := index
				if s > old {
					f(str[old:s-1], false)
				}
				f(str[s:e], true)
				old = e + 1
				continue
			}
		}
		break
	}
	if len(str) > old {
		f(str[old:], false)
	}
}

func IndexFrom(str, sep string, start *int) bool { // start with -1
	(*start)++
	if *start >= len(str) {
		return false
	}
	i := strings.Index(str[*start:], sep)
	if i != -1 {
		*start += i
		return true
	}
	return false
}

func LastByteAsString(str string) string {
	if len(str) == 0 {
		return ""
	}
	return string(str[len(str)-1])
}

func PadCamelCase(str, pad string) string {
	big := ""
	out := ""
	for _, r := range str {
		if r == unicode.ToUpper(r) {
			if big == "" && out != "" {
				out += pad
			}
			big += string(r)
		} else {
			if len(big) > 0 {
				if len(big) == 1 {
					out += big
				} else {
					if big == "URL" || big == "ID" {
						out += big
					} else {
						out += big[:len(big)-1]
						out += pad
						out += big[len(big)-1:]
					}
				}
				big = ""
			}
			out += string(r)
		}
	}
	out += big
	return out
}

func HashTo64Hex(str string) string {
	h := zint.HashTo64(str)
	return fmt.Sprintf("%x", h)
}

func HashTo32Hex(str string) string {
	h := zint.HashTo32(str)
	return fmt.Sprintf("%x", h)
}

func ReplaceSpaces(str string, rep rune) string {
	out := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return rep
		}
		return r
	}, str)

	return out
}

func CaselessCompare(a, b string) int {
	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func NumberToBase64Code(n int) string {
	const table = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	i := zint.Min(zint.Max(0, n), 63)
	return string(table[i])
}

func ArgsToString(args map[string]string, div, eq, quote string) string {
	var str string
	for k, v := range args {
		if str != "" {
			str += div
		}
		str += k + eq + quote + v + quote
	}
	return str
}

func GetArgsAsQuotedParameters(args map[string]string, div string) string {
	return ArgsToString(args, div, "=", `"`)
}

func GetArgsAsURLParameters(args map[string]string) string {
	vals := url.Values{}
	for k, v := range args {
		vals.Add(k, v)
	}
	return vals.Encode()
}

func GetParametersFromArgString(str, sep, set string) map[string]string {
	m := map[string]string{}
	for _, p := range strings.Split(str, sep) {
		var k, v string
		if SplitN(p, set, &k, &v) {
			k = strings.TrimSpace(k)
			v = strings.TrimSpace(v)
			m[k] = v
		}
	}
	return m
}

func GetParametersFromURLArgString(str string) map[string]string {
	return GetParametersFromArgString(str, "&", "=")
}

func Reverse(ss *[]string) {
	last := len(*ss) - 1
	for i := 0; i < len(*ss)/2; i++ {
		(*ss)[i], (*ss)[last-i] = (*ss)[last-i], (*ss)[i]
	}
}

func Reversed(ss []string) []string {
	end := len(ss)
	out := make([]string, end)
	for i := 0; i < end; i++ {
		out[end-i-1] = ss[i]
	}
	return out
}

func SplitIntoLengths(s string, length int) []string {
	sub := ""
	subs := []string{}

	runes := bytes.Runes([]byte(s))
	l := len(runes)
	for i, r := range runes {
		sub = sub + string(r)
		if (i+1)%length == 0 {
			subs = append(subs, sub)
			sub = ""
		} else if (i + 1) == l {
			subs = append(subs, sub)
		}
	}
	return subs
}

func FromInterface(i interface{}) string {
	s, _ := i.(string)
	return s
}

func SplitInTwo(str string, sep string) (string, string) {
	// TODO: Use strings.Cut
	parts := strings.SplitN(str, sep, 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}

/*
// ScanLinesWithCR is a replacement for bufio.Scanner
func ScanLinesWithCR(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	var cr bool
	for i := 0; i < len(data); i++ {
		if data[i] == '\r' {
			if i == len(data)-1 {
				return i + 1, data[0:i], nil
			}
			cr = true
		} else if data[i] == '\n' {
			end := i
			if cr {
				end--
			}
			return i + 1, data[0:end], nil
		} else {
			if cr {
				return i, data[0 : i-1], nil
			}
		}
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}
*/

func NewLineScanner(reader io.Reader, ctx context.Context, got func(line string, err error)) *bufio.Scanner {
	s := bufio.NewScanner(reader)
	go func() {
		for (ctx == nil || ctx.Err() == nil) && s.Scan() {
			if ctx == nil || ctx.Err() == nil {
				got(s.Text(), nil)
			}
		}
		err := s.Err()
		if err != nil {
			got("", err)
		}
	}()
	return s
}

func IsTypableASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < ' ' || c > unicode.MaxASCII {
			return false
		}
	}
	return true
}

func SortedMapKeys(m interface{}) (keys []string) {
	val := reflect.ValueOf(m)
	if val.Kind() != reflect.Map {
		panic("not map")
	}
	for _, key := range val.MapKeys() {
		str := fmt.Sprint(key)
		keys = append(keys, str)
	}
	sort.Strings(keys)
	return
}

func MatchWildcard(wild, str string) bool {
	if wild == "" {
		return str == wild
	}

	if wild == "*" {
		return true
	}
	// Does extended wildcard '*' and '?' match.
	return deepMatchRune([]rune(str), []rune(wild), false)
}

func deepMatchRune(str, wild []rune, simple bool) bool {
	for len(wild) > 0 {
		switch wild[0] {
		default:
			if len(str) == 0 || str[0] != wild[0] {
				return false
			}
		case '?':
			if len(str) == 0 && !simple {
				return false
			}
		case '*':
			return deepMatchRune(str, wild[1:], simple) ||
				(len(str) > 0 && deepMatchRune(str[1:], wild, simple))
		}

		str = str[1:]
		wild = wild[1:]
	}

	return len(str) == 0 && len(wild) == 0
}

var EscapeQuoteReplacer = strings.NewReplacer(
	"\r\n", "\\n",
	"\n", "\\n",
	"\t", "\\t",
	"\r", "\\n",
	"\"", "\\\"")

var UnEscapeQuoteReplacer = strings.NewReplacer(
	"\\n", "\n",
	"\\t", "\t",
	"\\\"", "\"")

var FileEscapeReplacer = strings.NewReplacer(
	"\\", "%5c", // `\` messes up this file's code formating
	"/", "%2f",
	":", "%3a",
)

var WhitespaceRemover = strings.NewReplacer(
	"\n", "",
	"\t", "",
	"\r", "",
	" ", "",
)

func ReplaceLinefeeds(str, with string) string {
	rep := strings.NewReplacer(
		"\r\n", with,
		"\n", with,
		"\r", with)
	return rep.Replace(str)
}

func Filter(slice []string, keep func(s string) bool) []string {
	n := make([]string, 0, len(slice))
	for _, s := range slice {
		if keep(s) {
			n = append(n, s)
		}
	}
	return n
}

func GetIDFromAnySliceItemWithIndex(a any, index int) string {
	getter, _ := a.(StrIDer)
	if getter != nil {
		return getter.GetStrID()
	}
	return strconv.Itoa(index)
}

var count int

func SplitStringWithDoubleAsEscape(str, split string) []string {
	count++
	const unlikely = "•°©°•"
	replacer := strings.NewReplacer(split+split, unlikely)
	deplacer := strings.NewReplacer(unlikely, split)
	if len(str) > 400 || len(split) > 200 {
		fmt.Println("SplitStringWithDoubleAsEscape:", str, split)
	}
	str = replacer.Replace(str)
	parts := strings.Split(str, split)
	for i, part := range parts {
		parts[i] = deplacer.Replace(part)
	}
	return parts
}

// SmartSort sorts a string slice using zstr.SmartCompare()
func SmartSort(s []string) {
	sort.Slice(s, func(i, j int) bool {
		return SmartCompare(s[i], s[j])
	})
}

// SmartCompare compares two strings, comparing as float if possible, or caseless
func SmartCompare(a, b string) bool {
	na, err := strconv.ParseFloat(a, 64)
	if err == nil {
		nb, err := strconv.ParseFloat(a, 64)
		if err == nil {
			return na < nb
		}
	}
	return CaselessCompare(a, b) < 0
}
