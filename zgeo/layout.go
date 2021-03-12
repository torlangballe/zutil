package zgeo

import (
	"github.com/torlangballe/zutil/zfloat"
)

type LayoutCell struct {
	Alignment         Alignment
	Margin            Size
	MaxSize           Size   // MaxSize is maximum size of child-view excluding margin
	MinSize           Size   // MinSize is minimum size of child-view excluding margin
	Collapsed         bool   // Collapsed is a cell that currently is not shown or takes up space.
	Free              bool   // Free Cells are placed using ContainerView method, not "inherited" ArrangeChildren method
	ExpandFromMinSize bool   // Makes cell expand into extra space in addition to minsize, not current size
	OriginalSize      Size   // Original size of object before layout
	Name              string // A name just for debugging
}

// stackCell is used to  calculate a box of *size* for each cell to layout in a stack.
// They are order by left, center, right alignment (x/y swapped and swapped back if vertical)
// Margin, Max, Min, OrignalSize are used to create these boxes.
// Then OriginalSize is aligned within each box to calculate each cell's final rectangle.
type stackCell struct {
	LayoutCell
	size       Size // size of box to align within
	added      bool // used during layout
	inputIndex int  // index of input cell
}

// getStackCells returns a slice of stackCells, in left, center, right order, minus collapsed.
// They are swapped if vertical for later to be swapped back.
// It creates the outRects slice, and lays out free cells within it, not returning them in scells
func getStackCells(r Rect, vertical bool, cells []LayoutCell) (scells []stackCell, outRects []Rect) {
	outRects = make([]Rect, len(cells), len(cells))
	for _, ca := range []Alignment{Left, HorCenter, Right} { // Add left ones, first, then center, then right
		for i, c := range cells {
			if c.Collapsed || c.Alignment == AlignmentNone {
				continue
			}
			a := c.Alignment
			// zlog.Info("performStacking add:", c.View.ObjectName(), a, ca)
			if vertical {
				a = a.Swapped()
			}
			if a&ca != 0 { // are we in left, center or right adding part now?
				var sc stackCell
				sc.LayoutCell = c
				sc.inputIndex = i
				// zlog.Info("Cell:", c.Name)
				if vertical {
					sc.Alignment = a // set alignment to the swapped version
					sc.Margin = c.Margin.Swapped()
					sc.MinSize = c.MinSize.Swapped()
					sc.MaxSize = c.MaxSize.Swapped()
					sc.OriginalSize = c.OriginalSize.Swapped()
				}
				sc.OriginalSize.MinimizeNonZero(sc.MinSize)
				// TODO: handle ExpandFromMinSize
				// TODO: If shrink align, shrink into rect
				sc.size = sc.OriginalSize
				sc.size.Add(sc.Margin)
				if c.Free {
					fr := r.Align(sc.size, sc.Alignment, sc.Margin, sc.MaxSize)
					if vertical {
						fr = fr.Swapped()
					}
					outRects[i] = fr
				} else {
					scells = append(scells, sc)
				}
			}
		}
	}
	return
}

// addLeftoverSpaceToWidths adjusts the size of scells to fit in total space in r.
// Each cell is increased based on it's current with compared to total
// it does two passes; First ones with a MaxSize or non-expanding, then rest with what is left.
// it returns *added*, which is sum of width of all added cells
func addLeftoverSpaceToWidths(r Rect, scells []stackCell, vertical bool, spacing float64) (added float64) {
	wspace := float64(len(scells)-1) * spacing // sum of all spacing between cells
	added = wspace
	for _, doLimited := range []bool{true, false} { // do limited width cells first, then ones that want as much as they can get
		var unaddedWidth float64
		for _, sc := range scells {
			if !sc.added {
				unaddedWidth += sc.size.W
			}
		}
		// if unaddedWidth == 0 {
		// 	break
		// }
		diff := r.Size.W - wspace - unaddedWidth
		// TODO: If diff < 0, and some cells have shrink align, shrink to sc.minsize
		for i, sc := range scells {
			// zlog.Info("SC:", sc.Name, sc.added, doCellsWithMaxW, sc.MaxSize.W)
			if sc.added {
				continue
			}
			if doLimited && (sc.MaxSize.W == 0 || sc.Alignment&HorExpand == 0) { // we're doing limited, and this one isn't
				continue
			}
			w := sc.size.W
			fractionOfUnadded := w / unaddedWidth // sets how much of total unadded width is width of this cell
			newWidth := w
			if sc.Alignment&HorExpand != 0 {
				newWidth += diff * fractionOfUnadded
			}
			if sc.MaxSize.W != 0 {
				zfloat.Maximize(&newWidth, sc.MaxSize.W+sc.Margin.W*2)
			}
			// TODO: handle proportional in first loop itteration
			scells[i].size.W = newWidth
			scells[i].added = true
			added += newWidth
			// zlog.Info("added:", r, sc.Name, doLimited, sc.Alignment, added, newWidth, w, fractionOfUnadded)
		}
	}
	return
}

// layoutRectsInBoxes goes left to right, making a box of full hight and each scell's width.
// it aligns the cell's original size within this with cells alignment.
// any space not used by cells (see jump below), is added between left and center, and center and right.
func layoutRectsInBoxes(r Rect, scells []stackCell, vertical bool, spacing, added float64, outRects []Rect) {
	x := r.Min().X
	prevAlign := Left
	jump := r.Size.W - added
	for i, sc := range scells {
		if (sc.Alignment&(Left|HorCenter|Right))&prevAlign == 0 { // if align is something new, ie. center/right
			if sc.Alignment&HorCenter != 0 { // if we are starting on center cells, add half of space left, and halve it for before right cells
				if i > 0 {
					x += jump / 2
					jump /= 2
				}
			} else { // we are starting with right cells, add rest of extra space
				x += jump
			}
			prevAlign = sc.Alignment
		}
		box := RectFromXYWH(x, r.Min().Y, sc.size.W, r.Size.H)
		// TODO: if sc.MaxSize.Y != 0 do something!!!
		// TODO: MarginIsOffset
		if i == len(scells)-1 {
			box.SetMaxX(r.Max().X)
		}
		vr := box.Align(sc.OriginalSize, sc.Alignment, sc.Margin, Size{})
		// zlog.Info(i, "align:", sc.Name, r, i, len(scells), box, sc.OriginalSize, sc.Alignment, "=", vr)
		x = vr.Max().X + spacing
		if vertical {
			vr = vr.Swapped()
			// sc.Alignment = sc.Alignment.Swapped() // these are just to debug print:
			// sc.Margin = sc.Margin.Swapped()
			// sc.MinSize = sc.MinSize.Swapped()
			// sc.MaxSize = sc.MaxSize.Swapped()
			// sc.OriginalSize = sc.OriginalSize.Swapped()
		}
		outRects[sc.inputIndex] = vr
		// zlog.Info("nstack:", r, sc.OriginalSize, sc.Name, box.Swapped(), vr, "max:", sc.MaxSize, "min:", sc.MinSize, sc.Alignment)
	}
}

// StackCells stacks cells horizontally or vertically in rect, returning resulting slice of rects in same slice positions as input cells.
func LayoutCellsInStack(rect Rect, vertical bool, spacing float64, cells []LayoutCell) []Rect {
	r := rect
	if vertical {
		r = r.Swapped() // we do everything as if it's a horizontal stack, swapping coordinates before and after if not
	}
	scells, outRects := getStackCells(r, vertical, cells)
	// zlog.Info("\nperformStacking2:", len(scells), rect, vertical)
	added := addLeftoverSpaceToWidths(r, scells, vertical, spacing)
	layoutRectsInBoxes(r, scells, vertical, spacing, added, outRects)

	return outRects
}
