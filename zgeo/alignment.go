package zgeo

import (
	"sort"
	"strings"
)

type Alignment uint64

const (
	AlignmentNone  = Alignment(0)
	Left           = Alignment(1)
	HorCenter      = Alignment(2)
	Right          = Alignment(4)
	Top            = Alignment(8)
	VertCenter     = Alignment(16)
	Bottom         = Alignment(32)
	HorExpand      = Alignment(64)
	VertExpand     = Alignment(128)
	HorShrink      = Alignment(256)
	VertShrink     = Alignment(512)
	HorOut         = Alignment(1024)
	VertOut        = Alignment(2048)
	Proportional   = Alignment(4096)
	HorJustify     = Alignment(8192)
	MarginIsOffset = Alignment(16384)
	ScaleToFitProp = Alignment(32768)

	Center     = HorCenter | VertCenter
	Expand     = HorExpand | VertExpand
	Shrink     = HorShrink | VertShrink
	HorScale   = HorExpand | HorShrink
	VertScale  = VertExpand | VertShrink
	Scale      = HorScale | VertScale
	Out        = HorOut | VertOut
	VertPos    = Top | VertCenter | Bottom
	HorPos     = Left | HorCenter | Right
	Vertical   = VertPos | VertScale | VertOut
	Horizontal = HorPos | HorScale | HorOut
)

var alignmentNames = map[string]Alignment{
	"none":           AlignmentNone,
	"left":           Left,
	"horCenter":      HorCenter,
	"right":          Right,
	"top":            Top,
	"vertCenter":     VertCenter,
	"bottom":         Bottom,
	"horExpand":      HorExpand,
	"vertExpand":     VertExpand,
	"horShrink":      HorShrink,
	"vertShrink":     VertShrink,
	"horOut":         HorOut,
	"vertOut":        VertOut,
	"proportional":   Proportional,
	"horJustify":     HorJustify,
	"marginIsOffset": MarginIsOffset,
	"scaleToFitProp": ScaleToFitProp,
}

func AlignmentFromVector(fromVector Pos) Alignment {
	//        a.init(rawValue rawFromVector(fromVector))
	return AlignmentNone
}

func (a Alignment) FlippedVertical() Alignment {
	var r = a
	r.AndWith(Horizontal)
	if a&Top != 0 {
		r.UnionWith(Bottom)
	}
	if a&Bottom != 0 {
		r.UnionWith(Top)
	}
	return r
}
func (a Alignment) FlippedHorizontal() Alignment {
	var r = a
	r.AndWith(Vertical)
	if a&Left != 0 {
		r.UnionWith(Right)
	}
	if a&Right != 0 {
		r.UnionWith(Left)
	}
	return r
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

func (a Alignment) String() string {
	var array []string

	center := (a&Center == Center)
	if center {
		array = append(array, "center")
	}
	for k, v := range alignmentNames {
		if center && v&Center != 0 {
			continue
		}
		if a&v != 0 {
			array = append(array, k)
		}
	}
	sort.Strings(array)

	return strings.Join(array, "|")
}

func AlignmentFromString(str string) Alignment {
	var a Alignment
	for _, s := range strings.Split(str, "|") {
		if s == "center" {
			a |= Center
		} else {
			a |= alignmentNames[s]
		}
	}
	return a
}

func (a Alignment) UnionWith(b Alignment) Alignment {
	return a | b
}

func (a Alignment) AndWith(b Alignment) Alignment {
	return a & b
}

func stringToRaw(str string) uint64 {
	var a = Alignment(0)
	for _, s := range strings.Split(str, " ") {
		a |= alignmentNames[s]
	}
	return uint64(a)
}

func rawFromVector(vector Pos) uint64 {
	var raw = Alignment(0)
	var angle = vector.ToAngleDeg()
	if angle < 0 {
		angle += 360
	}
	if angle < 45*0.5 {
		raw = Right
	} else if angle < 45*1.5 {
		raw = Right | Top
	} else if angle < 45*2.5 {
		raw = Top
	} else if angle < 45*3.5 {
		raw = Top | Left
	} else if angle < 45*4.5 {
		raw = Left
	} else if angle < 45*5.5 {
		raw = Left | Bottom
	} else if angle < 45*6.5 {
		raw = Bottom
	} else if angle < 45*7.5 {
		raw = Bottom | Right
	} else {
		raw = Right
	}
	return uint64(raw)
}
