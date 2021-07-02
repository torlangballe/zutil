package zgeo

import (
	"github.com/torlangballe/zutil/zbool"
)

type Alignment int32

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

var alignmentList = []zbool.BitsetItem{
	zbool.BSItem("none", int64(AlignmentNone)),
	zbool.BSItem("left", int64(Left)),
	zbool.BSItem("horcenter", int64(HorCenter)),
	zbool.BSItem("right", int64(Right)),
	zbool.BSItem("top", int64(Top)),
	zbool.BSItem("vertcenter", int64(VertCenter)),
	zbool.BSItem("bottom", int64(Bottom)),
	zbool.BSItem("horexpand", int64(HorExpand)),
	zbool.BSItem("vertexpand", int64(VertExpand)),
	zbool.BSItem("horshrink", int64(HorShrink)),
	zbool.BSItem("vertshrink", int64(VertShrink)),
	zbool.BSItem("horout", int64(HorOut)),
	zbool.BSItem("vertout", int64(VertOut)),
	zbool.BSItem("proportional", int64(Proportional)),
	zbool.BSItem("marginissoffset", int64(MarginIsOffset)),
	zbool.BSItem("center", int64(Center)),
	zbool.BSItem("expand", int64(Expand)),
	zbool.BSItem("shrink", int64(Shrink)),
	zbool.BSItem("scale", int64(Scale)),
}

func AlignmentFromString(str string) Alignment {
	return Alignment(zbool.StrToInt64FromList(str, alignmentList))
}
func (a *Alignment) FromStringToBits(str string) {
	*a = AlignmentFromString(str)
}

func (a Alignment) String() string {
	return zbool.Int64ToStringFromList(int64(a), alignmentList)
}

func (a *Alignment) UnmarshalJSON(b []byte) error {
	a.FromStringToBits(string(b))
	return nil
}

func (a Alignment) MarshalJSON() ([]byte, error) {
	return []byte(`"` + a.String() + `"`), nil
}

func AlignmentFromVector(fromVector Pos) Alignment {
	//        a.init(rawValue rawFromVector(fromVector))
	return AlignmentNone
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
