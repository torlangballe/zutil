package zgeo

import (
	"math"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zslice"
)

type LayoutCell struct {
	Alignment    Alignment // Alignment is how cell is placed within parent rect and siblings. If AlignmentNone, it is not placed at all
	Margin       Size      // Margin is the size around object in cell
	MaxSize      Size      // MaxSize is maximum size of object before margin. Can be only W or H
	MinSize      Size      // MinSize is minimum size of object before margin. Can be only W or H
	Collapsed    bool      // Collapsed is a cell that currently is not shown or takes up space.
	Free         bool      // Free Cells are placed using simple aligning to parent rect, not stacked etc
	OriginalSize Size      // Original size of object before layout
	Divider      float64   // This cell is a divider, wants its value subtracted from item before it, added to item after
	Name         string    // A name just for debugging
}

// stackCell is used to  calculate a box of *size* for each cell to layout in a stack.
// They are order by left, center, right alignment (x/y swapped and swapped back if vertical)
// Margin, Max, Min, OrignalSize are used to create these boxes.
// Then OriginalSize is aligned within each box to calculate each cell's final rectangle.
type stackCell struct {
	LayoutCell
	size Size // size of box to align within
	// added      bool // used during layout
	inputIndex int // index of input cell
}

// makeOutRectsAndFillFree creates a slice *out* for rects for each scell.
// It aligns the free cells and stores the rect in out, removing that cell from scells.
func fillFreeInOutRect(debugName string, r Rect, vertical bool, out []Rect, scells *[]stackCell) {
	for i := 0; i < len(*scells); {
		sc := (*scells)[i]
		if sc.Free {
			fr := r.AlignPro(sc.OriginalSize, sc.Alignment, sc.Margin, sc.MaxSize, sc.MinSize)
			// zlog.Info(r, "fillFreeInOutRect:", i, fr, sc.Name)
			if vertical {
				fr = fr.Swapped()
			}
			out[sc.inputIndex] = fr
			zslice.RemoveAt(scells, i)
			// zlog.Info("OR:", j, sc.Name, fr)
		} else {
			i++
		}
	}
	// for i, r := range out {
	// 	zlog.Info("fillFreeInOutRect:", i, r)
	// }
}

// getStackCells returns a slice of stackCells, in left, center, right order, minus collapsed.
// They are swapped if vertical for later to be swapped back.
// Free cells are added with original size
func getStackCells(debugName string, vertical bool, cells []LayoutCell) (scells []stackCell) {
	for _, ca := range []Alignment{Left, HorCenter, Right} { // Add left ones, first, then center, then right
		for i, c := range cells {
			if (c.Collapsed && !c.Free) || c.Alignment == AlignmentNone {
				continue
			}
			a := c.Alignment
			if vertical {
				a = a.Swapped()
			}
			if a&ca != 0 { // are we in left, center or right adding part now?
				var sc stackCell
				sc.inputIndex = i // important we do this now, when i is index of original input, scells will be sorted
				sc.LayoutCell = c
				if vertical {
					sc.Alignment = a // set alignment to the swapped version
					sc.Margin = c.Margin.Swapped()
					sc.MinSize = c.MinSize.Swapped()
					sc.MaxSize = c.MaxSize.Swapped()
					sc.OriginalSize = c.OriginalSize.Swapped()
				}
				// TODO: If shrink align, shrink into rect
				sc.size = sc.OriginalSize
				if c.Free {
					sc.size = c.OriginalSize
				} else {
					sc.size.Maximize(sc.MinSize)
					sc.size.MinimizeNonZero(sc.MaxSize)
				}
				// 	zlog.Info(debugName, i, sc.MinSize, sc.MaxSize, sc.size, "getStackCells add:", c.Alignment, c.Margin, c.Name, sc.OriginalSize)
				scells = append(scells, sc)
			}
		}
	}
	return
}

// calcPreAddedTotalWidth gets the total width of objects, spacing and margin
func calcPreAddedTotalWidth(debugName string, scells []stackCell, spacing float64) (w, space, marg float64) {
	for i, sc := range scells {
		w += sc.size.W
		marg += sc.Margin.W
		if sc.Alignment&HorCenter != 0 {
			marg += sc.Margin.W
		}
		if i != 0 {
			space += spacing
		}
		// zlog.Info("calcPreAddedTotalWidth:", sc.Name, sc.size, sc.Margin.W, spacing, w)
	}
	w += space + marg
	// zlog.Info("calcPreAddedTotalWidth total:", w)
	return
}

// getPossibleAdjustments returns how much each cell can shrink/expand, based on diff being + or -.
// MinSize, MaxSize and HorExpand alignment are used.
func getPossibleAdjustments(diff float64, scells []stackCell) []float64 {
	adj := make([]float64, len(scells), len(scells))
	for i, sc := range scells {
		w := sc.size.W
		if diff < 0 {
			if sc.MinSize.W != 0 {
				adj[i] = math.Min(0, sc.MinSize.W-w)
			}
			// zlog.Info(i, "pos:", diff, sc.Name, w, sc.MinSize.W, "adj", adj[i])
			continue
		}
		if sc.Alignment&HorExpand != 0 {
			if sc.MaxSize.W != 0 {
				adj[i] = math.Max(0, sc.MaxSize.W-w)
			} else {
				adj[i] = diff // we can take it all if needed
			}
			// zlog.Info(i, "pos+:", diff, sc.Name, w, sc.MaxSize.W, "adj", adj[i])
		}
	}
	return adj
}

// addLeftoverSpaceToWidths adjusts the size of scells to fit in total space in r.
// Each cell is increased based on its current width compared to total
// it does two passes; First ones with a MaxSize or non-expanding, then rest with what is left.
// it returns *added*, which is sum of width of all added cells
func addLeftoverSpaceToWidths(debugName string, r Rect, scells []stackCell, vertical bool, spacing float64) {
	width, _, _ := calcPreAddedTotalWidth(debugName, scells, spacing) // space, marg
	diff := r.Size.W - width
	adjustableCount := 0.0
	adj := getPossibleAdjustments(diff, scells)
	for _, a := range adj {
		if a != 0 {
			adjustableCount++
		}
	}
	// zlog.Info(adjustableCount, "addLeftoverSpaceToWidths:", debugName, r.Size.W, "diff:", diff, "spacing:", spacing, "w:", width)
	// zlog.Info("Possible adj:", adj)
	ndiff := diff
	//	viewsWidth := width - space - marg
	for {
		adjusted := false
		for i, sc := range scells {
			if adj[i] == 0 {
				continue
			}
			w := math.Max(1, sc.size.W)
			shouldAdjust := ndiff / adjustableCount
			// zlog.Info("adj:", shouldAdjust, ndiff, sc.Name, adj[i])
			//			if math.Abs(adj[i]) < math.Abs(shouldAdjust) {
			amount := math.Min(adj[i], shouldAdjust)
			if amount > 0 {
				// zlog.Info("  addLeftoverSpaceToWidths adjust:", amount, sc.Name, adj[i], ndiff)
				adjusted = true
				scells[i].size.W = w + adj[i]
				ndiff -= adj[i]
				adj[i] = 0
			}
		}
		diff = ndiff
		if !adjusted {
			break
		}
	}
	var biggiesTotalWidth float64
	for i, sc := range scells {
		if adj[i] != 0 {
			w := sc.size.W
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&w, sc.MaxSize.W)
			}
			biggiesTotalWidth += w
		}
	}
	zfloat.Maximize(&biggiesTotalWidth, 1)
	for i, sc := range scells {
		// zlog.Info("addLeftoverSpaceToWidths2:", sc.Name, adj[i], sc.MinSize, sc.MaxSize, sc.OriginalSize, sc.size)
		if adj[i] != 0 {
			w := sc.size.W
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&w, sc.MaxSize.W)
			}
			newWidth := scells[i].size.W + w/biggiesTotalWidth*diff
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&newWidth, sc.MaxSize.W)
			}
			// zlog.Info(vertical, biggiesTotalWidth, diff, "addLeftoverSpaceToWidths biggies:", sc.Name, w, newWidth, sc.MaxSize)
			scells[i].size.W = newWidth
		}
	}
	for i, sc := range scells {
		if sc.Divider != 0 {
			// zlog.Info("DIV0:", i, len(scells), sc.Divider, debugName, sc.Name)
			// zlog.Info("DIV1:", scells[i-1].Name, scells[i-1].size.W, scells[i+1].size.W, sc.Divider)
			scells[i-1].size.W += sc.Divider
			scells[i+1].size.W -= sc.Divider
			// zlog.Info("DIV2:", scells[i+1].Name, scells[i+1].size.W, scells[i+1].size.W, sc.Divider)
			break
		}
	}
}

// layoutRectsInBoxes goes left to right, making a box of full hight and each scell's width.
// it aligns the cell's original size within this with cells alignment.
// any space not used by cells (see jump below), is added between left and center, and center and right.
func layoutRectsInBoxes(debugName string, r Rect, scells []stackCell, vertical bool, spacing float64, outRects []Rect) {
	// if !vertical {
	// }
	sx := r.Min().X
	x := sx
	prevAlign := Left
	// lastCenterWidth := 0.0
	var wcenter, wright float64
	for i, sc := range scells {
		w := sc.size.W + sc.Margin.W
		if i != 0 {
			w += spacing
		}
		if sc.Alignment&HorCenter != 0 {
			wcenter += w
			// lastCenterWidth = w
		} else if sc.Alignment&Right != 0 {
			// wcenter += lastCenterWidth // when last center is done, we add its margin again for right side
			// lastCenterWidth = 0
			wright += w
		}
		// if debugName == "bt-bar" {
		// 	zlog.Info(i, "layoutRectsInBoxes:", sc.Name, w, sc.Alignment&HorCenter != 0, wcenter)
		// }
	}
	for i, sc := range scells {
		if (sc.Alignment&(Left|HorCenter|Right))&prevAlign == 0 { // if align is something new, ie. center/right
			if sc.Alignment&HorCenter != 0 { // if we are starting on center cells, add half of space left, and halve it for before right cells
				// x = math.Max(sx, r.Min().X+r.Size.W/2-wcenter/2)
				zfloat.Maximize(&x, math.Max(sx, r.Min().X+r.Size.W/2-wcenter/2))
			} else { // we are starting with right cells, add rest of extra space
				x = math.Max(sx, r.Max().X-wright)
			}
			prevAlign = sc.Alignment
		}
		w := sc.size.W + sc.Margin.W
		if sc.Alignment&HorCenter != 0 {
			w += sc.Margin.W
		}
		box := RectFromXYWH(x, r.Min().Y, w, r.Size.H)
		// TODO: if sc.MaxSize.Y != 0 do something!!!
		// TODO: MarginIsOffset
		if i == len(scells)-1 && (sc.Alignment&Right != 0) {
			box.SetMaxX(r.Max().X)
		}
		a := sc.Alignment
		if a&HorExpand == 0 {
			a |= HorShrink
		}
		if a&VertExpand == 0 {
			a |= VertShrink
		}
		vr := box.AlignPro(sc.OriginalSize, a, sc.Margin, sc.MaxSize, sc.MinSize)
		// zlog.Info("layoutRectsInBoxes:", box.Size, sc.OriginalSize, sc.Name, sc.Alignment, sc.MaxSize, vr)
		// zlog.Info("layout:", debugName, "rect:", r, sc.Name, "sc.size:", sc.size, sc.MinSize, "  align:", sc.Alignment, "box:", box, sc.OriginalSize, "=", vr)
		x = box.Max().X + spacing // was vr.Max!!!
		if vertical {
			vr = vr.Swapped()
		}
		outRects[sc.inputIndex] = vr
	}
}

// LayoutCellsInStack stacks cells horizontally or vertically in rect, returning resulting slice of rects in same slice positions as input cells.
func LayoutCellsInStack(debugName string, rect Rect, vertical bool, spacing float64, cells []LayoutCell) []Rect {
	// zlog.Info("LayoutCellsInStack", debugName, zlog.CallingStackString())
	// start := time.Now()
	r := rect
	if vertical {
		r = r.Swapped() // we do everything as if it's a horizontal stack, swapping coordinates before and after if not
	}
	scells := getStackCells(debugName, vertical, cells)
	outRects := make([]Rect, len(cells), len(cells))
	fillFreeInOutRect(debugName, r, vertical, outRects, &scells)
	addLeftoverSpaceToWidths(debugName, r, scells, vertical, spacing)
	layoutRectsInBoxes(debugName, r, scells, vertical, spacing, outRects)
	return outRects
}

func LayoutGetCellsStackedSize(debugName string, vertical bool, spacing float64, cells []LayoutCell) Size {
	scells := getStackCells(debugName, vertical, cells)
	// for i, sc := range scells {
	// 	zlog.Info("LayoutGetCellsStackedSize:", i, sc.Name, sc.OriginalSize, sc.size, sc.MinSize)
	// }
	w, _, _ := calcPreAddedTotalWidth(debugName, scells, spacing)
	// for i, sc := range scells {
	// 	zlog.Info(w, "LayoutGetCellsStackedSize preadded:", i, sc.Name, sc.OriginalSize, sc.size, sc.MinSize, sc.MaxSize)
	// }
	h := 0.0
	for _, sc := range scells {
		zfloat.Maximize(&h, sc.size.H+sc.Margin.H*2)
	}
	s := Size{w, h}
	if vertical {
		s = s.Swapped()
	}
	return s
}

// MaxCellsInSize returns how many cells with an cellSize, margin and spacing with fit in inSize
func MaxCellsInSize(spacing, margin, cellSize, inSize float64) int {
	return int((inSize - margin) / (spacing - 1 + cellSize))
}
