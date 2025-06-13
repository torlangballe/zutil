package zgeo

import (
	"math"

	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zslice"
)

type LayoutCell struct {
	Alignment        Alignment // Alignment is how cell is placed within parent rect and siblings. If AlignmentNone, it is not placed at all
	Margin           Rect      // Margin is the size around object in cell
	MaxSize          Size      // MaxSize is maximum size of object before margin. Can be only W or H
	MinSize          Size      // MinSize is minimum size of object before margin. Can be only W or H
	Collapsed        bool      // Collapsed is a cell that currently is not shown or takes up space.
	Free             bool      // Free Cells are placed using simple aligning to parent rect, not stacked etc
	OriginalSize     Size      // Original size of object before layout
	Divider          float64   // This cell is a divider, wants its value subtracted from item before it, added to item after
	RelativeToName   string    // If Free, and set, it is aligned to other cell with name
	ShowIfExtraSpace float64   // If set, show only if enough extra space left over
	Name             string    // A name for debugging/aligning
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

var LayoutDebugPrint bool

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
				// if debugName == "77950197" {
				// 	zlog.Info(debugName, i, sc.MinSize, sc.MaxSize, sc.size, "getStackCells add:", c.Alignment, c.Margin, c.Name, sc.OriginalSize)
				// }
				scells = append(scells, sc)
			}
		}
	}
	return
}

// calcPreAddedTotalWidth gets the total width of objects, spacing and margin
func calcPreAddedTotalWidth(debugName string, scells []stackCell, spacing float64) (w, space, marg float64) {
	// zlog.Info("calcPreAddedTotalWidth1:", debugName, spacing)
	for i, sc := range scells {
		if sc.ShowIfExtraSpace != 0 {
			continue
		}
		w += sc.size.W
		w -= sc.Margin.Size.W
		if sc.Alignment&HorCenter != 0 {
			w += sc.Margin.Size.W
		}
		if i != 0 {
			w += spacing
		}
		// zlog.Info("calcPreAddedTotalWidth:", debugName, sc.Name, sc.size, sc.Margin.W, w)
	}
	return
}

func calcMaxWidth(debugName string, scells []stackCell, spacing float64) float64 {
	// if debugName == "On" {
	// 	zlog.Info("calcMaxWidth cell1:", debugName)
	// }
	var w float64
	for i, sc := range scells {
		// if debugName == "$labelize.stack.Name Postfix" {
		// zlog.Info("calcMaxWidth:", sc.Name, sc.MaxSize, sc.Alignment)
		// }
		if sc.MaxSize.W == 0 {
			if sc.Alignment&HorExpand == 0 {
				w += sc.size.W
			} else {
				w = 0
				break
			}
		} else {
			w += sc.MaxSize.W
		}
		w += -sc.Margin.Size.W
		if sc.Alignment&HorCenter != 0 {
			w -= sc.Margin.Size.W
		}
		if i != 0 {
			w += spacing
		}
		// if debugName == "On" {
		// 	zlog.Info("calcMaxWidth cell:", debugName, sc.Name, sc.MaxSize, sc.size, "s:", w, sc.Alignment, "marg:", sc.Margin.W)
		// }
	}
	// if debugName == "On" {
	// 	zlog.Info("calcMaxWidth returned:", debugName, w)
	// }
	return w
}

// getPossibleAdjustments returns how much each cell can shrink/expand, based on diff being + or -.
// MinSize, MaxSize and HorExpand alignment are used.
func getPossibleAdjustments(diff float64, scells []stackCell) []float64 {
	adj := make([]float64, len(scells), len(scells))
	for i, sc := range scells {
		w := sc.size.W
		// zlog.Info(i, "getPossibleAdjustments:", diff, sc.Name, sc.Alignment, w, sc.MaxSize.W)
		if diff < 0 {
			if sc.MinSize.W != 0 {
				adj[i] = math.Min(0, sc.MinSize.W-w)
			}
			if sc.Alignment&HorShrink != 0 {
				adj[i] = diff
			}
			// zlog.Info(i, "pos:", diff, sc.Name, w, sc.MinSize.W, "adj", adj[i])
			continue
		}
		if sc.Alignment&HorExpand != 0 {
			if sc.MaxSize.W != 0 {
				adj[i] = math.Min(diff, math.Max(0, sc.MaxSize.W-w))
				// if adj[i] > 3000 {
				// zlog.Info(w, "ADJ!!:", diff, sc.Name, adj[i], sc.MaxSize.W)
				// }
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
func addLeftoverSpaceToWidths(debugName string, r Rect, scells *[]stackCell, vertical bool, spacing float64) {
	width, _, _ := calcPreAddedTotalWidth(debugName, *scells, spacing) // space, marg
	diff := r.Size.W - width

	var changed bool
	for i := 0; i < len(*scells); i++ {
		sc := (*scells)[i]
		if sc.ShowIfExtraSpace != 0 {
			if sc.ShowIfExtraSpace <= diff {
				(*scells)[i].ShowIfExtraSpace = 0
				changed = true
			} else {
				zslice.RemoveAt(scells, i)
				i--
			}
		}
	}
	if changed {
		width, _, _ = calcPreAddedTotalWidth(debugName, *scells, spacing) // space, marg
		diff = r.Size.W - width
	}

	adj := getPossibleAdjustments(diff, *scells) // this gives us how much each cell might be able to be added to. Note this is more than diff, so after adjusting one, ALL adj must be subtracted by amount added.
	// if debugName == "57178178" {
	// 	zlog.Info("addLeftoverSpaceToWidths:", r.Size.W, width, diff, adj)
	// }
	ndiff := diff
	for {
		adjustableCount := 0.0
		for _, a := range adj {
			if a != 0 {
				adjustableCount++
			}
		}
		adjusted := false
		for i, sc := range *scells {
			subtract := adj[i]
			if subtract == 0 {
				continue
			}
			w := math.Max(1, sc.size.W)
			shouldAdjust := ndiff / adjustableCount
			amount := zmath.AbsMin(subtract, shouldAdjust)
			if amount != 0 {
				if LayoutDebugPrint {
					zlog.Info("  addLeftoverSpaceToWidths adjust:", adjustableCount, amount, sc.Name, subtract, ndiff, adj)
				}
				adjustableCount--
				adjusted = true
				(*scells)[i].size.W = w + amount
				for j := i; j < len(adj); j++ {
					if adj[j] != 0 {
						adj[j] = adj[j] - amount
					}
				}
				ndiff -= amount
				continue
				//				adj[i] = 0
			}
		}
		diff = ndiff
		if !adjusted {
			break
		}
	}
	var biggiesTotalWidth float64
	for i, sc := range *scells {
		if adj[i] != 0 {
			w := sc.size.W
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&w, sc.MaxSize.W)
			}
			biggiesTotalWidth += w
		}
	}
	zfloat.Maximize(&biggiesTotalWidth, 1)
	for i, sc := range *scells {
		// zlog.Info("addLeftoverSpaceToWidths2:", sc.Name, adj[i], sc.MinSize, sc.MaxSize, sc.OriginalSize, sc.size)
		if adj[i] != 0 {
			w := sc.size.W
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&w, sc.MaxSize.W)
			}
			newWidth := (*scells)[i].size.W + w/biggiesTotalWidth*diff
			if sc.MaxSize.W != 0 {
				zfloat.Minimize(&newWidth, sc.MaxSize.W)
			}
			(*scells)[i].size.W = newWidth
		}
	}
	for i, sc := range *scells {
		if sc.Divider != 0 {
			div := sc.Divider
			div2 := div
			nsize := (*scells)[i-1].size.W + div
			if nsize < 20 {
				div -= (nsize - 20)
				div2 = 0
			}
			(*scells)[i-1].size.W += div
			(*scells)[i+1].size.W -= div2
			break
		}
	}
}

// layoutRectsInBoxes goes left to right, making a box of full hight and each scells' width.
// It aligns the cell's original size within this with cells alignment.
// Any space not used by cells, is added between left and center, and center and right.
func layoutRectsInBoxes(debugName string, r Rect, scells []stackCell, vertical bool, spacing float64, outRects []Rect) {
	// zlog.Info("layoutRectsInBoxes1:", debugName)
	// if !vertical {
	// }
	sx := r.Min().X
	x := sx
	prevAlign := Left
	// lastCenterWidth := 0.0
	var wcenter, wright float64
	for i, sc := range scells {
		w := sc.size.W - sc.Margin.Size.W
		if i != len(scells)-1 { // 0
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
		w := sc.size.W - sc.Margin.Size.W
		// if sc.Alignment&HorCenter != 0 {
		// 	w -= sc.Margin.Size.W
		// }
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
		// max := sc.MaxSize
		// if !a.Has(HorExpand) {
		// 	zfloat.Maximize(&max.W, sc.size.W)
		// }
		vr := box.AlignPro(sc.size, a, sc.Margin, sc.MaxSize, sc.MinSize)
		// if debugName == "57178178" {
		// 	zlog.Info("layout:", debugName, "rect:", r, sc.Name, "sc.size:", sc.size, sc.MinSize, "  align:", sc.Alignment, "box:", box, "orig:", sc.OriginalSize, "=", vr)
		// }
		x = box.Max().X + spacing // was vr.Max!!!
		if vertical {
			vr = vr.Swapped()
		}

		outRects[sc.inputIndex] = vr
	}
}

// LayoutCellsInStack stacks cells horizontally or vertically in rect, returning resulting slice of rects in same slice positions as input cells.
func LayoutCellsInStack(debugName string, rect Rect, vertical bool, spacing float64, cells []LayoutCell) []Rect {
	// start := time.Now()
	r := rect
	if vertical {
		r = r.Swapped() // we do everything as if it's a horizontal stack, swapping coordinates before and after if not
	}
	scells := getStackCells(debugName, vertical, cells)

	outRects := make([]Rect, len(cells), len(cells))
	fillFreeInOutRect(debugName, r, vertical, outRects, &scells)
	addLeftoverSpaceToWidths(debugName, r, &scells, vertical, spacing)

	layoutRectsInBoxes(debugName, r, scells, vertical, spacing, outRects)
	return outRects
}

func LayoutGetCellsStackedSize(debugName string, vertical bool, spacing float64, cells []LayoutCell) (s, max Size) {
	scells := getStackCells(debugName, vertical, cells)
	w, _, _ := calcPreAddedTotalWidth(debugName, scells, spacing)
	maxWidth := calcMaxWidth(debugName, scells, spacing)

	h := 0.0
	for _, sc := range scells {
		zfloat.Maximize(&h, sc.size.H-sc.Margin.Size.H)
	}
	s = SizeD(w, h)
	max = SizeD(maxWidth, 0)
	if vertical {
		s = s.Swapped()
		max = max.Swapped()
	}
	// if debugName == "License.sub" {
	// 	zlog.Info("LayoutGetCellsStackedSize done:", debugName, s, max)
	// }
	return s, max
}

// MaxCellsInLength returns how many cells with an cellSize and spacing with fit in inSize
func MaxCellsInLength(spacing, cellSize, inSize float64) int {
	return int(inSize / (spacing - 1 + cellSize))
}

func MaxCellsInSize(spacing, cellSize, inSize Size) (x, y int) {
	x = MaxCellsInLength(spacing.W, cellSize.W, inSize.W)
	y = MaxCellsInLength(spacing.H, cellSize.H, inSize.H)
	return
}
