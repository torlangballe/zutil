package zmath

import (
	"github.com/torlangballe/zutil/zslice"
)

type ParentChild interface {
	IdentifierOfParent() string
}

// We create a interface for Identifier for tree making, as zstr.StrIDer is usually for a the database id,
// whereas a child/parent-id might be based on something else
type Identifier interface {
	Identifier() string
}

type Leaf[T any] struct {
	Level    int
	Instance T
}

// MakeTree takes a slice of T where T implements Identifier, and some implement ParentChild.
// It returns a slice of Leaf[T] where the instances are ordered in a way that parents come before their children,
// and the Level field indicates the depth in the tree.
// The parent-child relationships are determined by the IdentifierOfParent method, which should return the identifier of the parent instance.
// The Identifier method should return a unique identifier for each instance.
// The function uses a recursive helper function addTree to build the tree structure.
func MakeTree[T any](rows []T) []Leaf[T] {
	out := addTree("", &rows, 0)
	for _, r := range rows { // clean up any remaining rows that were not added to the tree (e.g. because they had a parent that was not in the list)
		leaf := Leaf[T]{Level: 0, Instance: r}
		out = append(out, leaf)
	}
	return out
}

func addTree[T any](parentID string, rows *[]T, level int) []Leaf[T] {
	var out []Leaf[T]

	// zlog.Info("addTree1", len(*rows))
	for len(*rows) > 0 {
		var added bool
		for i := 0; i < len(*rows); i++ {
			var rpid string
			r := (*rows)[i]
			a := any(r)
			if pc, is := a.(ParentChild); is {
				rpid = pc.IdentifierOfParent()
			}
			id := a.(Identifier).Identifier()
			// zlog.Info(i, "addTree:", "parentID:", parentID, "id:", id, "rpid:", rpid, "level:", level)
			if rpid == parentID && (parentID != "" || level == 0) { // if parentID is empty, we want to include all top-level nodes (those with no parent), but empty==empty should otherwise not add anything
				added = true
				one := Leaf[T]{Level: level, Instance: r}
				zslice.RemoveAt(rows, i)
				add := addTree(id, rows, level+1)
				allAdded := append([]Leaf[T]{one}, add...)
				// zlog.Info("Added:", len(allAdded), len(add), "of", len(rows))
				out = append(out, allAdded...)
				zslice.RemoveFromFunc(rows, func(t T) bool {
					for _, leaf := range add { // remove add, "one" already removed
						if any(leaf.Instance).(Identifier).Identifier() == any(t).(Identifier).Identifier() {
							return true
						}
					}
					return false
				})
				i--
			}
		}
		// zlog.Info("addTree2:", len(rows), added, level)
		if !added {
			break
		}
	}
	return out
}
