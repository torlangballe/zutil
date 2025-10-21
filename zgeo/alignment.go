package zgeo

import (
	"github.com/torlangballe/zutil/zbits"
)

type Alignment int64

const (
	Left Alignment = 1 << iota
	Right
	Top
	HorCenter
	VertCenter
	Bottom
	HorExpand
	VertExpand
	HorShrink
	VertShrink
	HorOut
	VertOut

	Proportional
	MarginIsOffset // MarginIsOffset is a special flag used to offset a center alignment in absolute, instead of relative to sides.

	AlignmentNone Alignment = 0
	BottomLeft              = Bottom | Left
	BottomRight             = Bottom | Right
	BottomCenter            = Bottom | HorCenter
	TopLeft                 = Top | Left
	TopRight                = Top | Right
	TopCenter               = Top | HorCenter
	CenterLeft              = Left | VertCenter
	CenterRight             = Right | VertCenter

	Center    = HorCenter | VertCenter
	Expand    = HorExpand | VertExpand
	Shrink    = HorShrink | VertShrink
	HorScale  = HorExpand | HorShrink
	VertScale = VertExpand | VertShrink
	Scale     = HorScale | VertScale
	Out       = HorOut | VertOut

	VertPos    = Top | VertCenter | Bottom
	HorPos     = Left | HorCenter | Right
	Vertical   = VertPos | VertScale | VertOut
	Horizontal = HorPos | HorScale | HorOut
)

var nameMap = map[Alignment]string{
	AlignmentNone:  "none",
	Left:           "left",
	HorCenter:      "horcenter",
	Right:          "right",
	Top:            "top",
	VertCenter:     "vertcenter",
	Bottom:         "bottom",
	HorExpand:      "horexpand",
	VertExpand:     "vertexpand",
	HorShrink:      "horshrink",
	VertShrink:     "vertshrink",
	HorOut:         "horout",
	VertOut:        "vertout",
	Proportional:   "proportional",
	MarginIsOffset: "marginissoffset",
	Center:         "center",
	Expand:         "expand",
	Shrink:         "shrink",
	Scale:          "scale",
}

type VerticeFlag struct {
	Vertical   bool
	Horizontal bool
}

func (a Alignment) String() string {
	return zbits.BitsToStrings(a, nameMap)
}

func AlignmentFromString(str string) Alignment {
	return zbits.StringsToBits(str, nameMap)
}

func (a Alignment) Vector() Pos {
	if a&Left != 0 {
		return Pos{-1, 0}
	}
	if a&Right != 0 {
		return Pos{1, 0}
	}
	if a&Top != 0 {
		return Pos{0, -1}
	}
	if a&Bottom != 0 {
		return Pos{0, 1}
	}
	return Pos{}
}

func (a Alignment) FlippedVertical() Alignment {
	var r = a
	r = r.And(Horizontal)
	if a&Top != 0 {
		r = r.Union(Bottom)
	}
	if a&Bottom != 0 {
		r = r.Union(Top)
	}
	return r
}

func (a Alignment) FlippedHorizontal() Alignment {
	var r = a
	r = r.And(Vertical)
	if a&Left != 0 {
		r = r.Union(Right)
	}
	if a&Right != 0 {
		r = r.Union(Left)
	}
	return r
}

// Swapped returns a new alignment where all the vertical alignments are the equivalent of what the horizontal ones were, and visa versa.
func (a Alignment) Swapped() Alignment {
	var o = AlignmentNone
	if a&Left != 0 {
		o |= Top
	}
	if a&Right != 0 {
		o |= Bottom
	}
	if a&Top != 0 {
		o |= Left
	}
	if a&HorCenter != 0 {
		o |= VertCenter
	}
	if a&VertCenter != 0 {
		o |= HorCenter
	}
	if a&Bottom != 0 {
		o |= Right
	}
	if a&HorExpand != 0 {
		o |= VertExpand
	}
	if a&VertExpand != 0 {
		o |= HorExpand
	}
	if a&HorShrink != 0 {
		o |= VertShrink
	}
	if a&VertShrink != 0 {
		o |= HorShrink
	}
	if a&HorOut != 0 {
		o |= VertOut
	}
	if a&VertOut != 0 {
		o |= HorOut
	}
	return o
}

func (a Alignment) Has(mask Alignment) bool {
	return a&mask != 0
}

func (a Alignment) HasAll(mask Alignment) bool {
	return a&mask == mask
}

func (a Alignment) Subtracted(sub Alignment) Alignment {
	return Alignment(a & Alignment(^uint64(sub)))
}

func (a Alignment) Only(vertical bool) Alignment {
	if vertical {
		return a.Subtracted(Horizontal | HorExpand | HorShrink | HorOut)
	}
	return a.Subtracted(Vertical | VertExpand | VertShrink | VertOut)
}

func (a Alignment) Union(b Alignment) Alignment {
	return a | b
}

func (a Alignment) And(b Alignment) Alignment {
	return a & b
}

func rawFromVector(vector Pos) uint64 {
	var raw = Alignment(0)
	var angle = vector.ToAngleDeg()
	if angle < 0 {
		angle += 360
	}
	switch {
	case angle < 45*0.5:
		raw = Right
	case angle < 45*1.5:
		raw = Right | Top
	case angle < 45*2.5:
		raw = Top
	case angle < 45*3.5:
		raw = Top | Left
	case angle < 45*4.5:
		raw = Left
	case angle < 45*5.5:
		raw = Left | Bottom
	case angle < 45*6.5:
		raw = Bottom
	case angle < 45*7.5:
		raw = Bottom | Right
	default:
		raw = Right
	}
	return uint64(raw)
}

// For an Alignment that has multiple x/y alignments (i.e Left and Right),
// SplitIntoIndividual returns a slice of all combinations of them with only a single x/y combination each
func (a Alignment) SplitIntoIndividual() (all []Alignment) {
	mask := ^(TopLeft | Center | BottomRight | Out)
	outs := []Alignment{AlignmentNone}
	useOut := (a&Out != 0)
	if useOut {
		outs = []Alignment{HorOut, VertOut}
	}
	for _, o := range outs {
		for _, x := range []Alignment{Right, Left, HorCenter} {
			for _, y := range []Alignment{Top, Bottom, VertCenter} {
				if a&x != 0 && a&y != 0 && (a&o != 0 || !useOut) {
					all = append(all, (a&mask)|x|y|o)
				}
			}
		}
	}
	return
}

func (a *Alignment) UnmarshalJSON(b []byte) error {
	*a = AlignmentFromString(string(b))
	return nil
}

func (a Alignment) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.String() + `"`), nil
}
