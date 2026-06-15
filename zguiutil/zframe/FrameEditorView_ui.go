//go:build zui

package zframe

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
	"github.com/torlangballe/zutil/zslices"
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
	UpdateFunc         func(frames []Frame, temporary bool)
	frames             []Frame
	options            Options
	frameEditor        *zcustom.CustomView
	frameTable         *zslicegrid.TableView[Frame]
	size               zgeo.Size
	divSize            float64
	draggingFrame      *Frame
	draggingCorner     zgeo.Alignment
	centerDragFrame    Frame
	centerDragStart    zgeo.Pos
	freeCornerDragging bool

	selectedCorner  zgeo.Alignment
	selectedFrameID int64
	lastUpdate      time.Time
	callingUpdate   bool
}

var (
	frameCount int
	directions = []zgeo.Alignment{zgeo.TopLeft, zgeo.TopRight, zgeo.BottomLeft, zgeo.BottomRight}
)

func NewFrameEditorView(frames []Frame, options Options, size zgeo.Size) *FrameEditorView {
	v := &FrameEditorView{}
	v.frames = frames
	v.size = size
	v.divSize = 1
	if size.W >= 1300 {
		v.divSize = 2
	}
	v.StackView.Init(v, true, "editor")
	v.SetMargin(zgeo.RectFromMarginSize(zgeo.SizeBoth(10)))
	v.frameEditor = zcustom.NewView("frame-editor")
	v.frameEditor.SetBGColor(zgeo.ColorDarkGray)
	v.frameEditor.SetDrawHandler(v.draw)
	v.frameEditor.SetCanTabFocus(true)
	v.Add(v.frameEditor, zgeo.TopLeft)

	v.frameTable = zslicegrid.TableViewNew[Frame](&v.frames, "etheros.frame-editor", zslicegrid.AddHeader|zslicegrid.AllowEdit|zslicegrid.AllowDelete|zslicegrid.AllowDuplicate|zslicegrid.AddBarInHeader)
	v.Add(v.frameTable, zgeo.TopLeft|zgeo.Expand)
	v.frameTable.StructName = "Frame"
	v.frameTable.ActionMenu.CreateItemsFunc = func() []zmenu.MenuedOItem {
		selected := v.frameTable.Grid.SelectedIDsOrHoverIDOrAll()
		def := v.frameTable.CreateDefaultMenuItems(selected, false)
		return append(def,
			zmenu.MenuedSCFuncAction("Add Frame", 'N', 0, v.addFrame),
		)
	}
	v.frameTable.StoreChangedItemsFunc = func(frames []Frame) {
		v.frameTable.SetItemsInSlice(frames)
		v.frameTable.UpdateViewFunc(true, false)
		if v.UpdateFunc != nil {
			v.UpdateFunc(frames, false)
		}
		v.frameTable.Expose()
	}
	v.frameEditor.SetPointerEnterHandler(true, v.handlePointerEnter)
	v.frameEditor.SetPressUpDownMovedHandler(v.handlePressAndDrag)
	v.frameEditor.SetKeyHandler(v.handleKey)
	v.updateSize()
	return v
}

func (v *FrameEditorView) ReadyToShow(beforeWindow bool) {
	if beforeWindow {
		v.callUpdate()
	}
}

func (v *FrameEditorView) findFrameForID(id int64) *Frame {
	for i, frame := range v.frames {
		if frame.ID == v.selectedFrameID {
			return &v.frames[i]
		}
	}
	return nil
}

func (v *FrameEditorView) handleKey(km zkeyboard.KeyMod, down bool) bool {
	dir := zkeyboard.ArrowKeyToDirection(km.Key)
	if dir != zgeo.AlignmentNone {
		selectedFrame := v.findFrameForID(v.selectedFrameID)
		if selectedFrame != nil && v.selectedCorner != zgeo.AlignmentNone {
			freeDragging := (km.Modifier&zkeyboard.ModifierAlt != 0)
			fast := (km.Modifier&zkeyboard.ModifierShift != 0)
			vec := dir.Vector()
			if fast {
				vec.MultiplyD(4)
			}
			if v.selectedCorner == zgeo.Center {
				for _, dir := range directions {
					v.centerDragFrame.Corners[dir] = v.centerDragFrame.Corners[dir].Plus(vec)
				}
			} else {
				npos := selectedFrame.Corners[v.selectedCorner].Plus(vec)
				setFrameCornerPos(selectedFrame, v.selectedCorner, npos, freeDragging)
			}
			v.frameEditor.Expose()
			v.callUpdate()
			return true
		}
		return true
	}
	return false
}

func (v *FrameEditorView) dirForPos(frame *Frame, pos zgeo.Pos) zgeo.Alignment {
	for _, dir := range directions {
		if v.isNear(pos, frame.Corners[dir], 5) {
			return dir
		}
	}
	if v.scaledRectToView(frame.Bounds()).ExpandedD(-4).Contains(pos) {
		return zgeo.Center
	}
	return zgeo.AlignmentNone
}

func (v *FrameEditorView) handlePressAndDrag(pos zgeo.Pos, down zbool.BoolInd) bool {
	if down.IsTrue() {
		for i := range v.frames {
			frame := &v.frames[i]
			v.freeCornerDragging = (zkeyboard.ModifiersAtPress == zkeyboard.ModifierAlt)
			dir := v.dirForPos(frame, pos)
			if dir != zgeo.AlignmentNone {
				v.draggingCorner = dir
				v.selectedCorner = dir
				v.selectedFrameID = frame.ID
				if dir == zgeo.Center {
					v.centerDragFrame = *frame
					v.centerDragStart = pos
				}
				v.draggingFrame = frame
				return true
			}
		}
		return false
	}
	if down.IsFalse() {
		v.draggingCorner = zgeo.AlignmentNone
		v.frameEditor.Expose()
		v.frameEditor.SetCursor(zcursor.Pointer)
		v.draggingFrame = nil
		v.callUpdate()
		return false
	}
	if v.draggingFrame != nil && v.draggingCorner != zgeo.AlignmentNone {
		if v.draggingCorner == zgeo.Center {
			diff := v.viewToPos(pos.Minus(v.centerDragStart))
			for _, dir := range directions {
				p := v.centerDragFrame.Corners[dir]
				v.draggingFrame.Corners[dir] = p.Plus(diff)
			}
			v.centerDragStart = pos
		} else {
			npos := v.viewToPos(pos)
			setFrameCornerPos(v.draggingFrame, v.draggingCorner, npos, v.freeCornerDragging)
		}
		v.frameEditor.Expose()
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

func setFrameCornerPos(frame *Frame, corner zgeo.Alignment, pos zgeo.Pos, freeCornerDragging bool) {
	diff := pos.Minus(frame.Corners[corner])
	frame.Corners[corner] = pos
	if !freeCornerDragging {
		hflip := corner.FlippedHorizontal()
		frame.Corners[hflip] = frame.Corners[hflip].Plus(zgeo.PosD(0, diff.Y))
		vflip := corner.FlippedVertical()
		frame.Corners[vflip] = frame.Corners[vflip].Plus(zgeo.PosD(diff.X, 0))
	}
}

func (v *FrameEditorView) updateSize() {
	s := v.size.DividedByD(v.divSize)
	v.frameEditor.SetMinSize(s)
	if v.IsPresented() {
		zcontainer.ArrangeChildrenAtRootContainer(v, true)
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

func (v *FrameEditorView) addFrame() {
	// currentFrame := v.currentFrame()
	center := zgeo.Rect{Size: v.size}.Center()
	small := v.size.TimesD(0.3)
	rect := zgeo.RectFromCenterSize(center, small)
	if len(v.frames) > 0 {
		bounds := v.frames[len(v.frames)-1].Bounds()
		if bounds.Max().X < v.size.W-bounds.Size.W {
			rect = bounds
			rect.Pos.X = bounds.Max().X
		}
	}
	frame := FrameFromRect(rect)
	frame.Color = zslices.Random(defaultColors)
	frameCount++
	frame.Name = fmt.Sprint("Frame", frameCount)
	title := "Add New Frame"
	v.frameTable.EditItems([]Frame{frame}, title, true, true, func(ok bool) {
		v.frameEditor.Expose()
	})
	v.callUpdate()
}

func (v *FrameEditorView) drawGrabRect(canvas *zcanvas.Canvas, frame *Frame, corner zgeo.Alignment) {
	center := v.posToView(frame.Corners[corner])
	r := zgeo.RectFromWH(6, 6)
	r = r.Centered(center)
	path := zgeo.PathNewRect(r, zgeo.SizeNull)
	if frame.ID == v.selectedFrameID && corner == v.selectedCorner {
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
	for _, frame := range v.frames {
		path := zgeo.PathNew()
		col := frame.Color
		canvas.SetColor(col)
		path.MoveTo(v.posToView(frame.Corners[zgeo.TopLeft]))
		path.LineTo(v.posToView(frame.Corners[zgeo.TopRight]))
		path.LineTo(v.posToView(frame.Corners[zgeo.BottomRight]))
		path.LineTo(v.posToView(frame.Corners[zgeo.BottomLeft]))
		path.Close()
		canvas.StrokePath(path, 2, zgeo.PathLineRound)
		v.drawGrabRect(canvas, &frame, zgeo.TopLeft)
		v.drawGrabRect(canvas, &frame, zgeo.TopRight)
		v.drawGrabRect(canvas, &frame, zgeo.BottomLeft)
		v.drawGrabRect(canvas, &frame, zgeo.BottomRight)
		tinfo.Rect = v.rectToView(frame.Bounds()).ExpandedD(-5)
		tinfo.Text = frame.Name
		tinfo.Color = frame.Color
		tinfo.Draw(canvas)
	}
}

func (v *FrameEditorView) isNear(pointer, corner zgeo.Pos, radius float64) bool {
	return v.posToView(corner).Minus(pointer).Length() < radius
}

func (v *FrameEditorView) handlePointerEnter(pos zgeo.Pos, inside zbool.BoolInd) {
	for _, frame := range v.frames {
		dir := v.dirForPos(&frame, pos)
		if dir != zgeo.AlignmentNone {
			v.frameEditor.SetResizeCursorFromAlignment(dir)
			return
		}
	}
	v.frameEditor.SetCursor(zcursor.Pointer)
}

func (v *FrameEditorView) callUpdate() {
	go v.UpdateFunc(v.frames, true)
}
