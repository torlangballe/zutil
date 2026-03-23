package zmath

import (
	"fmt"
	"testing"

	"github.com/torlangballe/zutil/ztesting"
)

func TestTree(t *testing.T) {
	rows := []Hierary{
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
