package zhasis

type Class struct {
	ID    int64  `db:"id"`
	IsID  int64  `db:"isid"`
	Name  string `db:"name"`
	About string `db:"about"`
	Icon  string `db:"icon"`
}

type Instance struct {
	ID     int64  `db:"id"`
	OfID   int64  `db:"ofid"`
	Value  string `db:"value"`
	UserID int64  `db:"userid"`
}

type VerbName string
type Verb int

type Link struct {
	ID        int64
	Verb      VerbName
	ToClassID int64
}

type Relation struct {
	FromClassID int64
	LinkID      int64
}

type Value struct {
	FromInstanceID  int64
	LinkID          int64
	ValueInstanceID int64
	OverrideLinkID  int64
}

type Tree struct {
	ID       int64
	Children []Tree
}

const (
	VerbNone         VerbName = ""
	VerbWithin       VerbName = "within"        // is in of, physically or abstract. A chair is within a room.
	VerbOn           VerbName = "on"            // physically on
	VerbUnder        VerbName = "under"         // physically under, somewhat covered, opposite of on
	VerbAbove        VerbName = "above"         // physically above
	VerbBelow        VerbName = "below"         // physically below
	VerbLeftOf       VerbName = "left-of"       // physically left-of
	VerbRightOf      VerbName = "right-of"      // physically right-of
	VerbOwnedBy      VerbName = "owned-by"      // legally owned by, ownership
	VerbControlledBy VerbName = "controlled-by" // run by, operated by,
	VerbPartOf       VerbName = "part-of"       // a sub-component of
	VerbAttributeOf  VerbName = "attribute-of"  // a characteristic/attribute of something. ID, color etc
)

var numbersToVerbMap = map[Verb]VerbName{
	0:  VerbNone,
	1:  VerbWithin,
	2:  VerbOn,
	3:  VerbUnder,
	4:  VerbAbove,
	5:  VerbBelow,
	6:  VerbLeftOf,
	7:  VerbRightOf,
	8:  VerbOwnedBy,
	9:  VerbControlledBy,
	10: VerbPartOf,
	11: VerbAttributeOf,
}

var verbsToNumbersMap = func() map[VerbName]Verb {
	m := map[VerbName]Verb{}
	for n, v := range numbersToVerbMap {
		m[v] = n
	}
	return m
}()
