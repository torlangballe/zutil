package zstr

import (
	"fmt"
	"strings"
	"testing"
)

func TestWildCard(t *testing.T) {
	fmt.Println("TestWildCard")
	match := deepMatchRune([]rune("Delux Music"), []rune("*ani*"), false)
	if match {
		t.Error("shouldn't match")
	}
	match = deepMatchRune([]rune("Delux Music"), []rune("*Mu*"), false)
	if !match {
		t.Error("shouldn't match")
	}
}

func testPad(t *testing.T, in, want string) {
	out := PadCamelCase(in, " ")
	out = strings.Replace(out, "_", " ", -1)
	if out != want {
		t.Error("PadCamelCase wrong:", in, "want:", want, "got:", out)
	}
}

func TestPadCamelCase(t *testing.T) {
	fmt.Println("TestPadCamelCase")
	testPad(t, "BigBadWolf", "Big Bad Wolf")
	testPad(t, "QoE", "QoE")
	testPad(t, "Sjonvarp_Simans_HD", "Sjonvarp Simans HD")
}
