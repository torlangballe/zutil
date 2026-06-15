package zframe

import (
	"math/rand"
	"strconv"

	"github.com/torlangballe/zutil/zgeo"
)

type FrameCornerType string

type Frame struct {
	ID      int64                       `zui:"-"`
	Color   zgeo.Color                  `zui:"width:100"`
	Name    string                      `zui:"width:400"`
	Corners map[zgeo.Alignment]zgeo.Pos `zui:"-"`
}

func MakeFrame() Frame {
	var frame Frame
	frame.Corners = map[zgeo.Alignment]zgeo.Pos{}
	frame.ID = rand.Int63()
	return frame
}

func (f Frame) GetStrID() string {
	return strconv.FormatInt(f.ID, 10)
}

func (f Frame) Bounds() zgeo.Rect {
	r := zgeo.Rect{Pos: f.Corners[zgeo.TopLeft]}
	r.UnionWithPos(f.Corners[zgeo.TopRight])
	r.UnionWithPos(f.Corners[zgeo.BottomRight])
	r.UnionWithPos(f.Corners[zgeo.BottomLeft])
	return r
}

func FrameFromRect(rect zgeo.Rect) Frame {
	frame := MakeFrame()
	frame.Corners[zgeo.TopLeft] = rect.TopLeft()
	frame.Corners[zgeo.TopRight] = rect.TopRight()
	frame.Corners[zgeo.BottomLeft] = rect.BottomLeft()
	frame.Corners[zgeo.BottomRight] = rect.BottomRight()
	return frame
}
