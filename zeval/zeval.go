package zeval

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/ustr"
)

const (
	Error  = 1
	String = 2
	Number = 4
)

type Value struct {
	Type int
	Str  string
	Num  float64
}

func BoolValue(b bool) Value {
	var n float64
	if b {
		n = 1
	}
	return Value{Number, "", n}
}

type Operator struct {
	str      string
	function func(a, b Value, str string) Value
}

var operators = []Operator{
	Operator{"==", func(a, b Value, str string) Value {
		if a.Type == String {
			return BoolValue(a.Str == b.Str)
		}
		return BoolValue(a.Num == b.Num)
	}},
	Operator{"!=", func(a, b Value, str string) Value {
		if a.Type == String {
			return BoolValue(a.Str != b.Str)
		}
		return BoolValue(a.Num != b.Num)
	}},
	Operator{">=", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{Error, ">= on string", 0}
		}
		return BoolValue(a.Num >= b.Num)
	}},
	Operator{">", func(a, b Value, str string) Value { // Must be after >=
		if a.Type == String {
			return Value{Error, "> on string", 0}
		}
		return BoolValue(a.Num > b.Num)
	}},
	Operator{"<=", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{Error, "<= on string", 0}
		}
		return BoolValue(a.Num <= b.Num)
	}},
	Operator{"<", func(a, b Value, str string) Value { // must be after <=
		if a.Type == String {
			return Value{Error, "< on string", 0}
		}
		return BoolValue(a.Num < b.Num)
	}},
	Operator{"+", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{String, a.Str + b.Str, 0}
		}
		return Value{Number, "", a.Num + b.Num}
	}},
	Operator{" - ", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{Error, "- on string", 0}
		}
		return Value{Number, "", a.Num - b.Num}
	}},
	Operator{"*", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{Error, "* on string", 0}
		}
		return Value{Number, "", a.Num * b.Num}
	}},
	Operator{"/", func(a, b Value, str string) Value {
		if a.Type == String {
			return Value{Error, "/ on string", 0}
		}
		if b.Num == 0 {
			return Value{Error, "division by 0: " + str, 0}
		}
		return Value{Number, "", a.Num / b.Num}
	}},
}

func getBracketedParts(str string) (got bool, first, middle, last string, err error) {
	old1 := str
	first = ustr.ExtractStringTilSeparator(&str, "(")
	if old1 != str {
		old2 := str
		last = ustr.ExtractStringFromEndTilSeparator(&str, ")")
		if old2 == str {
			err = errors.New("No matching ')' after ')'")
			return
		}
		middle = str
		got = true
	}
	return
}

func getValueFromString(str string, variables *strings.Replacer) (val Value) {
	//	fmt.Println("getValueFromString:", str)
	for _, o := range operators {
		parts := strings.SplitN(str, o.str, 2)
		if len(parts) == 2 {
			//			fmt.Println("EVAL:", parts, o.str, str)
			v1 := getValueFromString(parts[0], variables)
			if v1.Type == Error {
				return v1
			}
			v2 := getValueFromString(parts[1], variables)
			if v1.Type == Error {
				return v2
			}
			if v1.Type != v2.Type {
				//				return Value{Error, fmt.Sprintln("operator on different types: ", str, " (", v1.Type, ",", v2.Type, ")"), 0}
			}
			return o.function(v1, v2, str)
		}
	}
	str = strings.TrimSpace(str)
	if strings.HasPrefix(str, `"`) && strings.HasSuffix(str, `"`) {
		return Value{String, str[1 : len(str)-1], 0}
	}
	str = variables.Replace(str)
	//	fmt.Println("getValueFromString2:", str)
	if str == "true" {
		return Value{Number, "", 1}
	}
	if str == "false" {
		return Value{Number, "", 0}
	}

	num, err := strconv.ParseFloat(str, 64)
	//	fmt.Println("valfromstr:", str, num, err)
	if err == nil {
		return Value{Number, "", num}
	}
	return Value{String, str, 0}
}

func EvaluateString(str string, variables *strings.Replacer) (value Value) {
	parts := strings.SplitN(str, "?", 2)
	if len(parts) == 2 {
		choices := strings.SplitN(str, ":", 2)
		if len(choices) != 2 {
			return Value{Error, "Evaluate: ? not followed by :", 0}
		}
		value = EvaluateString(parts[0], variables)
		if value.Type == Error {
			return
		}
		choiceString := choices[1]
		if value.Num == 1 {
			choiceString = choices[0]
		}
		return EvaluateString(choiceString, variables)
	}
	got, first, middle, last, err := getBracketedParts(str)
	if got {
		value = EvaluateString(middle, variables)
		switch value.Type {
		case Error:
			return
		case Number:
			middle = strconv.FormatFloat(value.Num, 'g', -1, 64)
		case String:
			middle = value.Str
		}
		str = first + middle + last
	} else if err != nil {
		return Value{Error, err.Error(), 0}
	}
	if testAndOr(&value, str, "||", variables) {
		return
	}
	if testAndOr(&value, str, "&&", variables) {
		return
	}
	return getValueFromString(str, variables)
}

func testAndOr(value *Value, str, operator string, variables *strings.Replacer) bool {
	vals := make([]bool, 2)
	parts := strings.Split(str, operator)
	if len(parts) != 2 {
		return false
	}
	for i, p := range parts {
		v := EvaluateString(p, variables)
		if v.Type == Error {
			*value = v
			return true
		}
		if v.Type != Number {
			(*value).Str = fmt.Sprintln("And/Or got non numeric arg", i, p)
			(*value).Type = Error
			return true
		}
		vals[i] = v.Num != 0
	}
	if operator == "||" {
		*value = BoolValue(vals[0] || vals[1])
	}
	if operator == "&&" {
		*value = BoolValue(vals[0] && vals[1])
	}
	return true
}

func MakeValueReplacer(vars map[string]string) *strings.Replacer {
	array := make([]string, len(vars)*2)
	i := 0
	for k, v := range vars {
		array[i] = k
		i++
		array[i] = v
		i++
	}
	return strings.NewReplacer(array...)
}
