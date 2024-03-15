package zframeeditor

import (
	"math/rand"
	"strconv"

	"github.com/torlangballe/zutil/zgeo"
)

type BoxCornerType string

type Box struct {
	ID      int64                       `zui:"-"`
	Color   zgeo.Color                  `zui:"width:100"`
	Name    string                      `zui:width:400"`
	Corners map[zgeo.Alignment]zgeo.Pos `zui:"-"`
}

func MakeBox() Box {
	var box Box
	box.Corners = map[zgeo.Alignment]zgeo.Pos{}
	box.ID = rand.Int63()
	return box
}

func (b Box) GetStrID() string {
	return strconv.FormatInt(b.ID, 10)
}

func (b Box) Bounds() zgeo.Rect {
	r := zgeo.Rect{Pos: b.Corners[zgeo.TopLeft]}
	r.UnionWithPos(b.Corners[zgeo.TopRight])
	r.UnionWithPos(b.Corners[zgeo.BottomRight])
	r.UnionWithPos(b.Corners[zgeo.BottomLeft])
	return r
}

func BoxFromRect(rect zgeo.Rect) Box {
	box := MakeBox()
	box.Corners[zgeo.TopLeft] = rect.TopLeft()
	box.Corners[zgeo.TopRight] = rect.TopRight()
	box.Corners[zgeo.BottomLeft] = rect.BottomLeft()
	box.Corners[zgeo.BottomRight] = rect.BottomRight()
	return box
}
