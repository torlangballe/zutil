package zgeo

import (
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zlog"
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

type stackCell struct {
	LayoutCell
	size       Size
	added      bool
	inputIndex int
}

func getUnaddedWidth(scells []stackCell) float64 {
	var w float64
	for _, sc := range scells {
		if !sc.added {
			w += sc.size.W
		}
	}
	return w
}

// StackCells stacks cells horizontally or vertically in rect, returning resulting slice of rects in same slice positions as input cells.
func LayoutCellsInStack(rect Rect, vertical bool, spacing float64, cells []LayoutCell) []Rect {
	var scells []stackCell // scells is slice of cells for stacking, in left-to-right order
	clen := len(cells)
	r := rect
	if vertical {
		r = r.Swapped() // we do everything as if it's a horizontal stack, swapping coordinates before and after if not
	}
	for _, ca := range []Alignment{Left, HorCenter, Right} { // Add left ones, first, then center, then right
		for i, c := range cells {
			if !c.Free && !c.Collapsed { // Skip ones that are collapsed or free, ie not stacked, but placed with base method
				a := c.Alignment
				// zlog.Info("performStacking add:", c.View.ObjectName(), a, ca)

				if vertical {
					a = a.Swapped()
				}
				if a&ca != 0 { // are we in left, center or right adding part now?
					var sc stackCell
					sc.LayoutCell = c
					sc.inputIndex = i
					zlog.Info("Cell:", c.Name)
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
					scells = append(scells, sc)
				}
			}
		}
	}
	zlog.Info("\nperformStacking2:", len(scells), rect, vertical)
	wspace := float64(len(scells)-1) * spacing
	added := wspace
	zlog.Info("WSPACE:", added)
	for _, doCellsWithMaxW := range []bool{true, false} {
		unaddedWidth := getUnaddedWidth(scells)
		if unaddedWidth == 0 {
			break
		}
		diff := r.Size.W - wspace - unaddedWidth
		// TODO: If diff < 0, and some cells have shrink align, shrink to sc.minsize
		for i, sc := range scells {
			zlog.Info("SC:", sc.Name, sc.added, doCellsWithMaxW, sc.MaxSize.W)
			if doCellsWithMaxW && (sc.MaxSize.W == 0 || sc.Alignment&HorExpand == 0) {
				continue
			}
			if sc.added {
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
			zlog.Info("added:", doCellsWithMaxW, sc.Alignment, added, newWidth, w, fractionOfUnadded)
		}
	}
	rects := make([]Rect, clen, clen)
	x := r.Min().X
	pa := Left
	jump := r.Size.W - added
	for i, sc := range scells {
		if (sc.Alignment&Left|HorCenter|Right)&pa == 0 {
			if sc.Alignment&HorCenter != 0 {
				zlog.Info("x add jump/2", jump/2, pa, sc.Alignment, added)
				x += jump / 2
				jump /= 2
			} else {
				zlog.Info("x add jump", jump, pa, sc.Alignment, added, wspace)
				x += jump
			}
			pa = sc.Alignment
		}
		box := RectFromXYWH(x, r.Min().Y, sc.size.W, r.Size.H)
		// TODO: if sc.MaxSize.Y != 0 do something!!!
		// TODO: MarginIsOffset
		if i == len(scells)-1 {
			box.SetMaxX(r.Max().X)
		}
		vr := box.Align(sc.OriginalSize, sc.Alignment, sc.Margin, Size{})
		zlog.Info("align:", r, i, len(scells), box, sc.OriginalSize, sc.Alignment, "=", vr)
		x = vr.Max().X + spacing
		if vertical {
			vr = vr.Swapped()
			sc.Alignment = sc.Alignment.Swapped() // these are just to debug print:
			sc.Margin = sc.Margin.Swapped()
			sc.MinSize = sc.MinSize.Swapped()
			sc.MaxSize = sc.MaxSize.Swapped()
			sc.OriginalSize = sc.OriginalSize.Swapped()
		}
		rects[sc.inputIndex] = vr
		zlog.Info("nstack:", r, sc.OriginalSize, sc.Name, box.Swapped(), vr, "max:", sc.MaxSize, "min:", sc.MinSize, sc.Alignment)
	}
	return rects
}
