package zstr

import (
	"fmt"
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
