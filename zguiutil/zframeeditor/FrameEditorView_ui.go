//go:build zui

package zframeeditor

import (
	"fmt"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zcursor"
	"github.com/torlangballe/zui/zcustom"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zui/zslicegrid"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zslice"
)

type Options int

const (
	AllowAngled Options = 1
	AllowName   Options = 2
	AllowColor  Options = 4
)

var defaultColors = []zgeo.Color{
	zgeo.ColorOrange,
	zgeo.ColorCyan,
	zgeo.ColorMagenta,
	zgeo.ColorRed,
	zgeo.ColorMaroon,
	zgeo.ColorYellow,
	zgeo.ColorOlive,
	zgeo.ColorLime,
	zgeo.ColorGreen,
	zgeo.ColorDarkGreen,
	zgeo.ColorAqua,
	zgeo.ColorTeal,
	zgeo.ColorBrown,
	zgeo.ColorBlue,
	zgeo.ColorNavy,
	zgeo.ColorFuchsia,
	zgeo.ColorPurple,
	zgeo.ColorPink,
}

type FrameEditorView struct {
	UpdateFunc func(boxes []Box, temporary bool)

	boxes   []Box
	options Options
	zcontainer.StackView
	boxEditor          *zcustom.CustomView
	boxTable           *zslicegrid.TableView[Box]
	size               zgeo.Size
	divSize            float64
	draggingBox        *Box
	draggingCorner     zgeo.Alignment
	centerDragBox      Box
	centerDragStart    zgeo.Pos
	freeCornerDragging bool
}

var (
	boxCount   int
	directions = []zgeo.Alignment{zgeo.TopLeft, zgeo.TopRight, zgeo.BottomLeft, zgeo.BottomRight}
)

func NewFrameEditorView(boxes []Box, options Options, size zgeo.Size) *FrameEditorView {
	v := &FrameEditorView{}
	v.boxes = boxes
	v.size = size
	v.divSize = 1
	if size.W >= 1300 {
		v.divSize = 2
	}
	v.StackView.Init(v, true, "editor")
	v.SetMargin(zgeo.RectFromMarginSize(zgeo.SizeBoth(10)))

	v.boxEditor = zcustom.NewView("box-editor")
	v.boxEditor.SetBGColor(zgeo.ColorDarkGray)
	v.boxEditor.SetDrawHandler(v.draw)
	v.Add(v.boxEditor, zgeo.TopLeft)

	v.boxTable = zslicegrid.TableViewNew[Box](&v.boxes, "etheros.box-editor", zslicegrid.AddHeader|zslicegrid.AllowEdit|zslicegrid.AllowDelete|zslicegrid.AllowDuplicate|zslicegrid.AddBarInHeader)
	v.Add(v.boxTable, zgeo.TopLeft|zgeo.Expand)
	v.boxTable.StructName = "Box"
	v.boxTable.ActionMenu.CreateItemsFunc = func() []zmenu.MenuedOItem {
		def := v.boxTable.CreateDefaultMenuItems()
		return append(def,
			zmenu.MenuedSCFuncAction("Add Box", 'N', 0, v.addBox),
		)
	}
	v.boxTable.StoreChangedItemsFunc = func(boxes []Box) {
		v.boxTable.SetItemsInSlice(boxes)
		v.boxTable.UpdateViewFunc()
		if v.UpdateFunc != nil {
			v.UpdateFunc(boxes, false)
		}
	}
	v.boxEditor.SetPointerEnterHandler(true, v.handlePointerEnter)
	v.boxEditor.SetPressUpDownMovedHandler(v.handlePressAndDrag)

	v.updateSize()
	return v
}

func (v *FrameEditorView) dirForPos(box *Box, pos zgeo.Pos) zgeo.Alignment {
	for _, dir := range directions {
		if v.isNear(pos, box.Corners[dir], 5) {
			return dir
		}
	}
	if v.scaledRectToView(box.Bounds()).ExpandedD(-4).Contains(pos) {
		return zgeo.Center
	}
	return zgeo.AlignmentNone
}

func (v *FrameEditorView) handlePressAndDrag(pos zgeo.Pos, down zbool.BoolInd) bool {
	if down.IsTrue() {
		for i := range v.boxes {
			box := &v.boxes[i]
			v.freeCornerDragging = (zkeyboard.ModifiersAtPress == zkeyboard.ModifierAlt)
			dir := v.dirForPos(box, pos)
			if dir != zgeo.AlignmentNone {
				v.draggingCorner = dir
				if dir == zgeo.Center {
					v.centerDragBox = *box
					v.centerDragStart = pos
				}
				v.draggingBox = box
				return true
			}
		}
		return false
	}
	if down.IsFalse() {
		v.draggingCorner = zgeo.AlignmentNone
		v.boxEditor.Expose()
		v.boxEditor.SetCursor(zcursor.Pointer)
		v.callUpdate()
		v.draggingBox = nil
		return false
	}
	if v.draggingBox != nil && v.draggingCorner != zgeo.AlignmentNone {
		if v.draggingCorner == zgeo.Center {
			diff := v.viewToPos(pos.Minus(v.centerDragStart))
			for _, dir := range directions {
				p := v.centerDragBox.Corners[dir]
				v.draggingBox.Corners[dir] = p.Plus(diff)
			}
			v.centerDragStart = pos
		} else {
			npos := v.viewToPos(pos)
			v.draggingBox.Corners[v.draggingCorner] = npos
			if !v.freeCornerDragging {
				hflip := v.draggingCorner.FlippedHorizontal()
				p := v.draggingBox.Corners[hflip]
				p.Y = npos.Y
				v.draggingBox.Corners[hflip] = p
				vflip := v.draggingCorner.FlippedVertical()
				p = v.draggingBox.Corners[vflip]
				p.X = npos.X
				v.draggingBox.Corners[vflip] = p
			}
		}
		v.boxEditor.Expose()
	}
	return true
}

func (v *FrameEditorView) updateSize() {
	s := v.size.DividedByD(v.divSize)
	v.boxEditor.SetMinSize(s)
	if v.IsPresented() {
		zcontainer.ArrangeChildrenAtRootContainer(v)
	}
}

func (v *FrameEditorView) scaledRectToView(r zgeo.Rect) zgeo.Rect {
	r.SetMin(v.posToView(r.Min()))
	r.SetMax(v.posToView(r.Max()))
	return r
}

func (v *FrameEditorView) posToView(pos zgeo.Pos) zgeo.Pos {
	return pos.DividedByD(v.divSize)
}

func (v *FrameEditorView) viewToPos(pos zgeo.Pos) zgeo.Pos {
	return pos.TimesD(v.divSize)
}

func (v *FrameEditorView) addBox() {
	// currentBox := v.currentBox()
	center := zgeo.Rect{Size: v.size}.Center()
	small := v.size.TimesD(0.3)
	rect := zgeo.RectFromCenterSize(center, small)
	if len(v.boxes) > 0 {
		bounds := v.boxes[len(v.boxes)-1].Bounds()
		if bounds.Max().X < v.size.W-bounds.Size.W {
			rect = bounds
			rect.Pos.X = bounds.Max().X
		}
	}
	box := BoxFromRect(rect)
	box.Color = zslice.Random(defaultColors)
	boxCount++
	box.Name = fmt.Sprint("Box", boxCount)
	title := "Add New Box"
	v.boxTable.EditItems([]Box{box}, title, true, true, func(ok bool) {
		v.boxEditor.Expose()
	})
	v.callUpdate()
}

func drawGrabRect(canvas *zcanvas.Canvas, center zgeo.Pos) {
	r := zgeo.RectFromWH(6, 6)
	r = r.Centered(center)
	canvas.SetColor(zgeo.ColorBlack)
	path := zgeo.PathNewRect(r, zgeo.Size{})
	canvas.StrokePath(path, 3, zgeo.PathLineSquare)

	r = zgeo.RectFromWH(5, 5)
	r = r.Centered(center)
	canvas.SetColor(zgeo.ColorWhite)
	path = zgeo.PathNewRect(r, zgeo.Size{})
	canvas.StrokePath(path, 1, zgeo.PathLineSquare)
}

func (v *FrameEditorView) draw(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
	for _, b := range v.boxes {
		path := zgeo.PathNew()
		col := b.Color
		canvas.SetColor(col)
		path.MoveTo(v.posToView(b.Corners[zgeo.TopLeft]))
		path.LineTo(v.posToView(b.Corners[zgeo.TopRight]))
		path.LineTo(v.posToView(b.Corners[zgeo.BottomRight]))
		path.LineTo(v.posToView(b.Corners[zgeo.BottomLeft]))
		path.Close()
		canvas.StrokePath(path, 2, zgeo.PathLineRound)

		drawGrabRect(canvas, v.posToView(b.Corners[zgeo.TopLeft]))
		drawGrabRect(canvas, v.posToView(b.Corners[zgeo.TopRight]))
		drawGrabRect(canvas, v.posToView(b.Corners[zgeo.BottomLeft]))
		drawGrabRect(canvas, v.posToView(b.Corners[zgeo.BottomRight]))
	}
}

func (v *FrameEditorView) isNear(pointer, corner zgeo.Pos, radius float64) bool {
	return v.posToView(corner).Minus(pointer).Length() < radius
}

func (v *FrameEditorView) handlePointerEnter(pos zgeo.Pos, inside zbool.BoolInd) {
	for _, box := range v.boxes {
		dir := v.dirForPos(&box, pos)
		if dir != zgeo.AlignmentNone {
			v.boxEditor.SetResizeCursorFromAlignment(dir)
			return
		}
	}
	v.boxEditor.SetCursor(zcursor.Pointer)
}

func (v *FrameEditorView) callUpdate() {
	go v.UpdateFunc(v.boxes, true)
}
