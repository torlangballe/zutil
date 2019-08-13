package ustr

import (
	"bytes"
	"crypto/md5"
	cryptoRand "crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"unicode"
	"unicode/utf8"

	"github.com/torlangballe/zutil/uinteger"
	"github.com/torlangballe/zutil/zmath"
)

var EscBlack string = "\x1B[30m"
var EscRed string = "\x1B[31m"
var EscGreen string = "\x1B[32m"
var EscYellow string = "\x1B[33m"
var EscBlue string = "\x1B[34m"
var EscMagenta string = "\x1B[35m"
var EscCyan string = "\x1B[36m"
var EscWhite string = "\x1B[37m"
var EscNoColor = "\x1b[0m"
var EscBlink string = "\x1b[5m"
var EscBlinkOff string = "\x1b[25m"
var EscYellowBlink string = "\033[5m"

func GetColorEscapeCode(r, g, b int) string {
	R := r&128 > 0
	G := g&128 > 0
	B := b&128 > 0
	if R && G && B {
		return EscWhite
	}
	if !R && !G && !B {
		return EscBlack
	}
	if R && !G && !B {
		return EscRed
	}
	if !R && G && !B {
		return EscGreen
	}
	if !R && !G && B {
		return EscBlue
	}
	if R && G && !B {
		return EscYellow
	}
	if R && !G && B {
		return EscMagenta
	}
	if !R && G && B {
		return EscCyan
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	} // if

	return b
}

func GetLevenshteinRatio(a, b string) float64 { // returns distance / min length of a or b
	len := float64(uinteger.IntMin(len(a), len(b)))
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
			mn := min(f[j]+1, f[j-1]+1) // delete & insert
			if cb != ca {
				mn = min(mn, fj1+1) // change
			} else {
				mn = min(mn, fj1) // matched
			} // else

			fj1, f[j] = f[j], mn // save f[j] to fj1(j is about to increase), update f[j] to mn
			j++
		} // for cb
	} // for ca

	return f[len(f)-1]
}

type MoreLines struct {
	height int
	index  int
	header string
	Writer *tabwriter.Writer
}

func NewMoreLines(h int, header string, writer *tabwriter.Writer) MoreLines {
	m := MoreLines{}
	m.height = h
	m.header = header
	m.Writer = writer

	return m
}

func WriteColoredHeaderToTabWriter(writer *tabwriter.Writer, col string, words ...string) {
	for _, w := range words {
		fmt.Fprint(writer, col, w, "\t")
	}
	fmt.Fprintln(writer, EscNoColor)
}

func (m *MoreLines) Check(quit *bool, typed *string) bool {
	if m.index == 0 && m.Writer != nil && m.header != "" {
		fmt.Fprintf(m.Writer, EscGreen+"\n"+m.header+EscWhite+"\n")
	}
	if m.index >= m.height {
		var sline string
		if m.Writer != nil {
			m.Writer.Flush()
		}
		fmt.Print(EscYellow + "press key or q and <return>:" + EscWhite)
		fmt.Scan(&sline)
		if sline == "q" {
			*quit = true
		}
		if typed != nil {
			*typed = sline
		}
		m.index = 0
		return false
	}
	m.index++
	return true
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

func HeadUntilString(str, sep string) string {
	i := strings.Index(str, sep)
	if i == -1 {
		return str
	}
	return str[:i]
}

func HeadUntilStringWithRest(str, sep string, rest *string) string {
	i := strings.Index(str, sep)
	if i == -1 {
		return str
	}
	*rest = str[i+len(sep):]
	return str[:i]
}

func Tail(str string, length int) string {
	l := zmath.IntClamp(length, 0, len(str)) // hack without unicode support
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

func TruncatedFromStart(str string, length int, endString string) string {
	l := zmath.IntClamp(length, 0, len(str)-len(endString)) // hack without unicode support
	return endString + str[l:]
}

func ConcatenateNonEmpty(divider string, parts ...string) (str string) {
	for _, s := range parts {
		if s != "" {
			if str == "" {
				str = s
			} else {
				str += divider + s
			}
		}
	}
	return
}

// Adds not-empty strings to str with divider. No divider on fidst add if initial str is empty.
func Concat(str *string, divider string, parts ...interface{}) {
	for _, p := range parts {
		s := fmt.Sprintf("%v", p)
		if s != "" {
			if *str == "" {
				*str = s
			} else {
				*str += divider + s
			}
		}
	}
	return
}

func StrIndexInStrings(str string, strs []string) int {
	for i, s := range strs {
		//		fmt.Print("StrIndexInStrings: '", str, "' : '", s, "'\n")
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

func ExtractStringTilSeparator(str *string, sep string) (got string) {
	for {
		i := strings.Index(*str, sep)
		switch i {
		//		case 0:
		//			*str = (*str)[len(sep):]
		case -1:
			return
		default:
			got = (*str)[:i]
			*str = (*str)[i+len(sep):]
			return
		}
	}
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

func RemoveStringFromSlice(base []string, str string) (result []string) {
	i := StrIndexInStrings(str, base)
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
		if StrIndexInStrings(s, sub) == -1 {
			result = append(result, s)
		}
	}
	return
}

func UnionStringSet(aset, bset []string) []string {
	for _, b := range bset {
		if StrIndexInStrings(b, aset) == -1 {
			aset = append(aset, b)
		}
	}
	return aset
}

func SlicesIntersect(aset, bset []string) bool {
	for _, b := range bset {
		if StrIndexInStrings(b, aset) != -1 {
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
		if StrIndexInStrings(b, aset) == -1 {
			return false
		}
	}
	return true
}

func AddToStringSet(strs *[]string, str string) bool {
	i := StrIndexInStrings(str, *strs)
	if i == -1 {
		*strs = append(*strs, str)
		return true
	}
	return false
}

func GenerateRandomBytes(count int) []byte {
	data := make([]byte, count)
	n, err := io.ReadFull(cryptoRand.Reader, data)
	if n != len(data) || err != nil {
		panic(err)
	}
	return data
}

func GenerateRandomHex(byteCount int) string {
	data := GenerateRandomBytes(byteCount)
	return hex.EncodeToString(data)
}

func GenerateUUID() string {
	data := GenerateRandomBytes(16)
	data[8] = 0x80
	data[4] = 0x40

	return hex.EncodeToString(data)
}

func MD5Hex(str string) string {
	return fmt.Sprintf("%x", md5.Sum([]byte(str)))
}

func Sha1Hex(str string) string {
	return fmt.Sprintf("%x", sha1.Sum([]byte(str)))
}

func JoinStringMap(m map[string]string, equal, sep string) (str string) {
	for key, val := range m {
		if str != "" {
			str += sep
		}
		str += key + equal + val
	}
	return
}

func RandomKVFromSSMap(m map[string]string) (string, string) {
	n := int(rand.Int31n(int32(len(m))))
	i := 0
	for k, v := range m {
		if i == n {
			return k, v
		}
		i++
	}
	panic("RandomKVFromSSMap outside")

	return "", ""
}

func AreStringMapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func GetKeysFromSIMapSorted(m map[string]interface{}) (keys []string) {
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return
}

func CountWords(str string) int {
	return len(strings.Fields(str)) // we can make this better at some point
}

func HasPrefix(str, prefix string, rest *string) bool {
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

func HasPrefixNoCase(str, prefix string, rest *string) bool {
	return HasPrefix(strings.ToLower(str), strings.ToLower(prefix), rest)
}

func HasSuffixNoCase(str, suffix string, rest *string) bool {
	return HasSuffix(strings.ToLower(str), strings.ToLower(suffix), rest)
}

func MakeCount(word string, count float64, langCode, plural string, significant int) string { // maybe just make the plural mandetory
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

func StrToBool(str string, def bool) bool {
	if str == "1" || str == "true" || str == "TRUE" {
		return true
	}
	if str == "0" || str == "false" || str == "FALSE" {
		return false
	}
	return def
}

func BoolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func GetQuotedArgs(args string) (parts []string) {
	RangeQuotedText(args, `"`, func(s string, inQuote bool) {
		if inQuote {
			parts = append(parts, s)
		} else if s != "" {
			s = strings.TrimSpace(s)
			if s != "" {
				split := strings.Split(strings.TrimSpace(s), " ")
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

func ReplaceVariablesWithValues(text, prefix string, values map[string]string) (content string) {
	spairs := make([]string, len(values)*2+2)
	keys := make([]string, 0, len(values))
	j := 0
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys) // sorts the keys

	for i := len(keys) - 1; i >= 0; i-- { // gets them in reverse, so StoryStartHour before StoryStart
		//		fmt.Println("ReplaceVariablesWithValues key:", keys[i])
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

func ReplaceString(str *string, find, set string) bool {
	text := strings.Replace(*str, find, set, -1)
	if *str != text {
		*str = text
		return true
	}
	return false
}

func FilterCharsFromEditor(r rune) rune {
	switch {
	case r == 0xA0:
		return ' '
	}
	return r
}

func ToAsciiCode(r rune) rune {
	switch {
	case r <= 127:
		return r
	case r == 0xA0:
		return ' '
	}
	return '•'
}

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
	return MakeCount(s, v, langCode, "", maxSignificant)
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
	return MakeCount(s, v, langCode, "", maxSignificant)
}

func SplitByNewLines(str string, skipEmpty bool) []string {
	str = strings.Replace(str, "\r\n", "\n", -1)
	str = strings.Replace(str, "\r", "\n", -1)
	lines := strings.Split(str, "\n")
	if !skipEmpty {
		return lines
	}
	nlines := make([]string, 0, len(lines))
	for _, l := range lines {
		if l != "" {
			nlines = append(nlines, l)
		}
	}
	return nlines
}

func RangeStringLines(str string, skipEmpty bool, f func(s string)) {
	for _, l := range SplitByNewLines(str, skipEmpty) {
		f(l)
	}
}

// func GetStringSliceFromIntBase10(ids []int64) []string {
// 	s := make([]string, len(ids))
// 	for i := range ids {
// 		s[i] = strconv.FormatInt(ids[i], 10)
// 	}
// 	return s
// }

func FirstToUpper(str string) (out string) {
	r := []rune(str)
	r[0] = unicode.ToUpper(r[0])
	out = string(r)
	return
}

func FirstToLower(str string) (out string) {
	r := []rune(str)
	r[0] = unicode.ToLower(r[0])
	out = string(r)
	return
}

func IsFirstLetterLowerCase(str string) bool {
	r, _ := utf8.DecodeRuneInString(str)
	return unicode.ToLower(r) == r
}

func IsFirstLetterUpperCase(str string) bool {
	r, _ := utf8.DecodeRuneInString(str)
	return unicode.ToUpper(r) == r
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

func LastLetter(str string) string {
	if len(str) == 0 {
		return ""
	}
	return string(str[len(str)-1])
}

func PadCamelCase(str, pad string) string {
	out := ""
	was := true
	for _, r := range str {
		if r == unicode.ToUpper(r) {
			if !was {
				out += pad
			}
			was = true
		} else {
			was = false
		}
		out += string(r)
	}
	return out
}

var hashRegEx = regexp.MustCompile(`#([A-Za-z_]\w+)`) // (\s|\B) at start needed?

func ReplaceHashTags(text string, f func(tag string) string) string {
	out := hashRegEx.ReplaceAllStringFunc(text, func(tag string) string {
		tag = strings.Replace(tag, "#", "", 1)
		return f(tag)
	})
	return out
}

func HashTo64Hex(str string) string {
	h := uinteger.HashTo64(str)
	return fmt.Sprintf("%x", h)
}

func HashTo32Hex(str string) string {
	h := uinteger.HashTo32(str)
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
	i := uinteger.IntMin(uinteger.IntMax(0, n), 63)
	return string(table[i])
}

func GetArgsAsQuotedParameters(args map[string]string, div string) string {
	str := ""
	for k, v := range args {
		if str != "" {
			str += div
		}
		str += k + `="` + v + `"`
	}
	return str
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

func Reverse(ss *[]string) {
	last := len(*ss) - 1
	for i := 0; i < len(*ss)/2; i++ {
		(*ss)[i], (*ss)[last-i] = (*ss)[last-i], (*ss)[i]
	}
}

func NiceFloat(f float64, significant int) string {
	format := fmt.Sprintf("%%.%df", significant)
	s := fmt.Sprintf(format, f)
	if strings.ContainsRune(s, '.') {
		HasSuffix(s, "0", &s)
		HasSuffix(s, "0", &s)
		HasSuffix(s, "0", &s)
		HasSuffix(s, "0", &s)
		HasSuffix(s, "0", &s)
		HasSuffix(s, "0", &s)
		HasSuffix(s, ".", &s)
	}
	return s
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
	`\`, "%5c",
	"/", "%2f",
	":", "%3a",
)
