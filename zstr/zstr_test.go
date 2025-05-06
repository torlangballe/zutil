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

func equal(t *testing.T, a any, eq string, descs ...any) {
	sa := fmt.Sprint(a)
	if sa != eq {
		str := Spaced(descs...) + fmt.Sprintf(": '%s' != '%s'", sa, eq)
		fmt.Sprintln("Fail:", str)
		t.Error(str)
	}
}

func TestRemovedFromSet(t *testing.T) {
	fmt.Println("TestRemovedFromSet")
	set := []string{"dog", "banana", "fish", "cat"}
	n := RemovedFromSet(set, "banana")
	fmt.Println("Removed1", n)
	equal(t, n, "[dog fish cat]", "banana not removed")
	n2 := RemovedFromSet(n, "dog", "cat")
	equal(t, n2, "[fish]", "not just fish")
}
