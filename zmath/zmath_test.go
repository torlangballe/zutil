package zmath

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/ztesting"
)

type Hierarchy struct {
	ID       string
	ParentID string
}

func (h Hierarchy) Identifier() string {
	return h.ID
}

func (h Hierarchy) IdentifierOfParent() string {
	return h.ParentID
}

func TestTree(t *testing.T) {
	rows := []Hierarchy{
		{ID: "Animal", ParentID: ""},
		{ID: "Mammal", ParentID: "Animal"},
		{ID: "Bird", ParentID: "Animal"},
		{ID: "Dog", ParentID: "Mammal"},
		{ID: "Cat", ParentID: "Mammal"},
		{ID: "Monkey", ParentID: "Mammal"},
		{ID: "TomCat", ParentID: "Cat"},
		{ID: "Chimpanzee", ParentID: "Monkey"},
	}
	tree := MakeTree(rows)
	str := fmt.Sprint(tree)
	ztesting.Equal(t, str, "[{0 {Animal }} {1 {Mammal Animal}} {2 {Dog Mammal}} {2 {Cat Mammal}} {3 {TomCat Cat}} {2 {Monkey Mammal}} {3 {Chimpanzee Monkey}} {1 {Bird Animal}}]")
}

func TestEaseInOut(t *testing.T) {
	ztesting.NearEqualF(t, EaseInOut(0), 0.0, "EaseInOut")
	ztesting.NearEqualF(t, EaseInOut(0.5), 0.5, "EaseInOut")
	ztesting.NearEqualF(t, EaseInOut(1), 1.0, "EaseInOut")
}
