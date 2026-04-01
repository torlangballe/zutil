package zmath

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/ztesting"
)

type Hierarchy struct {
	ID       string
	ParentID string
	Instance string
}

func (h Hierarchy) Identifier() string {
	return h.ID
}

func (h Hierarchy) ParentIdentifier() string {
	return h.ParentID
}

func TestTree(t *testing.T) {
	rows := []Hierarchy{
		{ID: "Animal", ParentID: "", Instance: ""},
		{ID: "Mammal", ParentID: "Animal", Instance: ""},
		{ID: "Bird", ParentID: "Animal", Instance: ""},
		{ID: "Dog", ParentID: "Mammal", Instance: ""},
		{ID: "Cat", ParentID: "Mammal", Instance: ""},
		{ID: "Monkey", ParentID: "Mammal", Instance: ""},
		{ID: "TomCat", ParentID: "Cat", Instance: ""},
		{ID: "Chimpanzee", ParentID: "Monkey", Instance: ""},
	}
	tree := MakeTree(rows)
	str := fmt.Sprint(tree)
	ztesting.Equal(t, str, "[{Animal 1 } {Mammal 2 } {Dog 3 } {Cat 3 } {TomCat 4 } {Monkey 3 } {Chimpanzee 4 } {Bird 2 }]")
}

func TestEaseInOut(t *testing.T) {
	ztesting.NearEqualF(t, EaseInOut(0), 0.0, "EaseInOut")
	ztesting.NearEqualF(t, EaseInOut(0.5), 0.5, "EaseInOut")
	ztesting.NearEqualF(t, EaseInOut(1), 1.0, "EaseInOut")
}
