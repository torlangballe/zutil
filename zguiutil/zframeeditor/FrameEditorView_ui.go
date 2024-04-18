//go:build zui

package zframeeditor

import (
	"fmt"
	"time"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zcursor"
	"github.com/torlangballe/zui/zcustom"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zui/zslicegrid"
	"github.com/torlangballe/zui/ztextinfo"
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
	zcontainer.StackView
	UpdateFunc         func(boxes []Box, temporary bool)
	boxes              []Box
	options            Options
	boxEditor          *zcustom.CustomView
	boxTable           *zslicegrid.TableView[Box]
	size               zgeo.Size
	divSize            float64
	draggingBox        *Box
	draggingCorner     zgeo.Alignment
	centerDragBox      Box
	centerDragStart    zgeo.Pos
	freeCornerDragging bool

	selectedCorner zgeo.Alignment
	selectedBoxID  int64
	lastUpdate     time.Time
	callingUpdate  bool
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
	v.boxEditor.SetCanTabFocus(true)
	v.Add(v.boxEditor, zgeo.TopLeft)

	v.boxTable = zslicegrid.TableViewNew[Box](&v.boxes, "etheros.box-editor", zslicegrid.AddHeader|zslicegrid.AllowEdit|zslicegrid.AllowDelete|zslicegrid.AllowDuplicate|zslicegrid.AddBarInHeader)
	v.Add(v.boxTable, zgeo.TopLeft|zgeo.Expand)
	v.boxTable.StructName = "Box"
	v.boxTable.ActionMenu.CreateItemsFunc = func() []zmenu.MenuedOItem {
		def := v.boxTable.CreateDefaultMenuItems(false)
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
		v.boxTable.Expose()
	}
	v.boxEditor.SetPointerEnterHandler(true, v.handlePointerEnter)
	v.boxEditor.SetPressUpDownMovedHandler(v.handlePressAndDrag)
	v.boxEditor.SetKeyHandler(v.handleKey)
	v.updateSize()
	return v
}

func (v *FrameEditorView) ReadyToShow(beforeWindow bool) {
	if beforeWindow {
		v.callUpdate()
	}
}

func (v *FrameEditorView) findBoxForID(id int64) *Box {
	for i, box := range v.boxes {
		if box.ID == v.selectedBoxID {
			return &v.boxes[i]
		}
	}
	return nil
}

func (v *FrameEditorView) handleKey(km zkeyboard.KeyMod, down bool) bool {
	dir := zkeyboard.ArrowKeyToDirection(km.Key)
	if dir != zgeo.AlignmentNone {
		selectedBox := v.findBoxForID(v.selectedBoxID)
		if selectedBox != nil && v.selectedCorner != zgeo.AlignmentNone {
			freeDragging := (km.Modifier&zkeyboard.ModifierAlt != 0)
			fast := (km.Modifier&zkeyboard.ModifierShift != 0)
			vec := dir.Vector()
			if fast {
				vec.MultiplyD(4)
			}
			if v.selectedCorner == zgeo.Center {
				for _, dir := range directions {
					v.centerDragBox.Corners[dir] = v.centerDragBox.Corners[dir].Plus(vec)
				}
			} else {
				npos := selectedBox.Corners[v.selectedCorner].Plus(vec)
				setBoxCornerPos(selectedBox, v.selectedCorner, npos, freeDragging)
			}
			v.boxEditor.Expose()
			v.callUpdate()
			return true
		}
		return true
	}
	return false
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
				v.selectedCorner = dir
				v.selectedBoxID = box.ID
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
		v.draggingBox = nil
		v.callUpdate()
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
			setBoxCornerPos(v.draggingBox, v.draggingCorner, npos, v.freeCornerDragging)
		}
		v.boxEditor.Expose()
		if !v.callingUpdate && time.Since(v.lastUpdate) > time.Millisecond*100 {
			v.callingUpdate = true
			v.lastUpdate = time.Now()
			v.callUpdate()
			v.lastUpdate = time.Now()
			v.callingUpdate = false
		}
	}
	return true
}

func setBoxCornerPos(box *Box, corner zgeo.Alignment, pos zgeo.Pos, freeCornerDragging bool) {
	diff := pos.Minus(box.Corners[corner])
	box.Corners[corner] = pos
	if !freeCornerDragging {
		hflip := corner.FlippedHorizontal()
		box.Corners[hflip] = box.Corners[hflip].Plus(zgeo.PosD(0, diff.Y))
		vflip := corner.FlippedVertical()
		box.Corners[vflip] = box.Corners[vflip].Plus(zgeo.PosD(diff.X, 0))
	}
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

func (v *FrameEditorView) rectToView(r zgeo.Rect) zgeo.Rect {
	return r.DividedByD(v.divSize)
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

func (v *FrameEditorView) drawGrabRect(canvas *zcanvas.Canvas, box *Box, corner zgeo.Alignment) {
	center := v.posToView(box.Corners[corner])
	r := zgeo.RectFromWH(6, 6)
	r = r.Centered(center)
	path := zgeo.PathNewRect(r, zgeo.SizeNull)
	if box.ID == v.selectedBoxID && corner == v.selectedCorner {
		canvas.SetColor(zgeo.ColorWhite)
		canvas.FillPath(path)
		return
	}
	canvas.SetColor(zgeo.ColorBlack)
	canvas.StrokePath(path, 3, zgeo.PathLineSquare)

	r = zgeo.RectFromWH(5, 5)
	r = r.Centered(center)
	canvas.SetColor(zgeo.ColorWhite)
	path = zgeo.PathNewRect(r, zgeo.SizeNull)
	canvas.StrokePath(path, 1, zgeo.PathLineSquare)
}

func (v *FrameEditorView) draw(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
	tinfo := ztextinfo.New()
	tinfo.Font = zgeo.FontNice(20, zgeo.FontStyleBold)
	tinfo.Alignment = zgeo.Center | zgeo.Shrink
	tinfo.MinimumFontScale = 0.2
	for _, box := range v.boxes {
		path := zgeo.PathNew()
		col := box.Color
		canvas.SetColor(col)
		path.MoveTo(v.posToView(box.Corners[zgeo.TopLeft]))
		path.LineTo(v.posToView(box.Corners[zgeo.TopRight]))
		path.LineTo(v.posToView(box.Corners[zgeo.BottomRight]))
		path.LineTo(v.posToView(box.Corners[zgeo.BottomLeft]))
		path.Close()
		canvas.StrokePath(path, 2, zgeo.PathLineRound)
		v.drawGrabRect(canvas, &box, zgeo.TopLeft)
		v.drawGrabRect(canvas, &box, zgeo.TopRight)
		v.drawGrabRect(canvas, &box, zgeo.BottomLeft)
		v.drawGrabRect(canvas, &box, zgeo.BottomRight)
		tinfo.Rect = v.rectToView(box.Bounds()).ExpandedD(-5)
		tinfo.Text = box.Name
		tinfo.Color = box.Color
		tinfo.Draw(canvas)
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
