package ustr

import (
	"testing"
)

const prefix = "$"

func TestReplaceVariablesWithValues(t *testing.T) {
	var res string
	text1 := "This is a test for $reason."
	res1 := "This is a test for something."

	text2 := "This is a test for $anotherreason."
	res2 := "This is a test for something cool."

	values := map[string]string{"reason": "something",
		"anotherreason": "something $adj",
		"adj":           "cool"}

	// single replace
	res = ReplaceVariablesWithValues(text1, prefix, values)
	if res != res1 {
		t.Fatalf("Output did not match: was '%s' but should have been '%s'",
			res, res1)
	}

	// replace with value that itself has a variable in it
	res = ReplaceVariablesWithValues(text2, prefix, values)
	if res != res2 {
		t.Fatalf("Output did not match: was '%s' but should have been '%s'",
			res, res2)
	}
}
