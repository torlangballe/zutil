package zgeo

import (
	"image"
	"math"
)

type Rect struct {
	Pos  Pos  `json:"pos"`
	Size Size `json:"size"`
}

var (
	RectUndef = RectFromWH(math.MaxFloat32, math.MaxFloat32)
	RectNull  Rect
)

func RectMake(x0, y0, x1, y1 float64) Rect {
	r := Rect{}
	r.Pos.X = x0
	r.Pos.Y = y0
	r.Size.W = x1 - x0
	r.Size.H = y1 - y0
	return r
}

func RectFromXYWH(x, y, w, h float64) Rect {
	return Rect{Pos{x, y}, Size{w, h}}
}

func RectFromMinMax(min, max Pos) Rect {
	return Rect{min, max.Minus(min).Size()}
}

func RectFromCenterSize(center Pos, size Size) Rect {
	return Rect{Size: size}.Centered(center)
}

func RectFromXY2(x, y, x2, y2 float64) Rect {
	return Rect{Pos{x, y}, Size{x2 - x, y2 - y}}
}

func RectFromWH(w, h float64) Rect {
	return Rect{Size: Size{w, h}}
}

func (r Rect) IsNull() bool {
	return r.Pos.X == 0 && r.Pos.Y == 0 && r.Size.W == 0 && r.Size.H == 0
}

func (r Rect) IsUndef() bool {
	return r.Size.W == math.MaxFloat32
}

func (r Rect) GoRect() image.Rectangle {
	return image.Rectangle{Min: r.Min().GoPoint(), Max: r.Max().GoPoint()}
}

func RectFromGoRect(rect image.Rectangle) Rect {
	return RectFromMinMax(PosFromGoPoint(rect.Min), PosFromGoPoint(rect.Max))
}

func (r Rect) TopLeft() Pos     { return r.Min() }
func (r Rect) TopRight() Pos    { return Pos{r.Max().X, r.Min().Y} }
func (r Rect) BottomLeft() Pos  { return Pos{r.Min().X, r.Max().Y} }
func (r Rect) BottomRight() Pos { return r.Max() }

func (r Rect) Max() Pos {
	return Pos{r.Pos.X + r.Size.W, r.Pos.Y + r.Size.H}
}

func (r *Rect) SetMax(max Pos) {
	r.Size.W = max.X - r.Pos.X
	r.Size.H = max.Y - r.Pos.Y
}

// func (r *Rect) SetMaxAsPos(max Pos) {
// 	r.Size.W = max.X - r.Pos.X
// 	r.Size.H = max.Y - r.Pos.Y
// }

func (r Rect) Min() Pos {
	return r.Pos
}

func (r *Rect) SetMin(min Pos) {
	r.Size.W += (r.Pos.X - min.X)
	r.Size.H += (r.Pos.Y - min.Y)
	r.Pos = min
}

func (r *Rect) SetMaxX(x float64) {
	r.Size.W = x - r.Pos.X
}

func (r *Rect) SetMaxY(y float64) {
	r.Size.H = y - r.Pos.Y
}

func (r *Rect) SetMinX(x float64) {
	r.Size.W += (r.Pos.X - x)
	r.Pos.X = x
}

func (r *Rect) SetMinY(y float64) {
	r.Size.H += (r.Pos.Y - y)
	r.Pos.Y = y
}

func (r Rect) Center() Pos {
	return r.Pos.Plus(r.Size.DividedByD(2).Pos())
}

func (r *Rect) SetCenter(c Pos) {
	r.Pos = c.Minus(r.Size.Pos().DividedByD(2))
}

func MergeAll(rects []Rect) []Rect {
	var merged = true
	var rold = rects
	for merged {
		var rnew []Rect
		merged = false
		for i, r := range rold {
			var used = false
			for j := i + 1; j < len(rold); j++ {
				if r.Overlaps(rold[j].ExpandedD(4)) {
					var n = rects[i]
					n = n.UnionedWith(rold[j])
					rnew = append(rnew, n)
					merged = true
					used = true
				}
			}
			if !used {
				rnew = append(rnew, r)
			}
		}
		rold = rnew
	}
	return rold
}

func (r Rect) ExpandedD(n float64) Rect {
	return r.Expanded(SizeBoth(n))
}

func (r Rect) Centered(center Pos) Rect {
	return Rect{center.Minus(r.Size.Pos().DividedByD(2)), r.Size}
}

func (r Rect) Expanded(s Size) Rect {
	r2 := Rect{Pos: r.Pos.Minus(s.Pos()), Size: r.Size.Plus(s.TimesD(2))}
	// zlog.Info("r.Expand:", r, s, r2)
	return r2
}

// AlignmentTransform moves right if Left, left if Right, or shrinks in Center
func (r Rect) AlignmentTransform(s Size, a Alignment) Rect {
	if a&Left != 0 {
		r.Pos.X += s.W
	} else if a&Right != 0 {
		r.Pos.X -= s.W
	} else if a&HorCenter != 0 {
		r.SetMinX(r.Min().X + s.W)
		r.SetMaxX(r.Max().X - s.W)
	}
	if a&Top != 0 {
		r.Pos.Y += s.H
	} else if a&Bottom != 0 {
		r.Pos.Y -= s.H
	} else if a&VertCenter != 0 {
		r.SetMinY(r.Min().Y + s.H)
		r.SetMaxY(r.Max().Y - s.H)
	}
	return r
}

func (r Rect) Overlaps(rect Rect) bool {
	min := r.Min()
	max := r.Max()
	rmin := rect.Min()
	rmax := rect.Max()
	return rmin.X < max.X && rmin.Y < max.Y && rmax.X > min.X && rmax.Y > min.Y
}

func (r Rect) Intersected(rect Rect) Rect {
	max := r.Max().Min(rect.Max())
	min := r.Min().Max(rect.Min())
	return RectFromMinMax(min, max)
}

func (r Rect) Contains(pos Pos) bool {
	min := r.Min()
	max := r.Max()
	return pos.X >= min.X && pos.X <= max.X && pos.Y >= min.Y && pos.Y <= max.Y
}

func (r Rect) Align(s Size, align Alignment, marg Size) Rect {
	return r.AlignPro(s, align, marg, Size{}, Size{})
}

func (r Rect) AlignPro(s Size, align Alignment, marg, maxSize, minSize Size) Rect {
	var x float64
	var y float64
	var scalex float64
	var scaley float64

	var wa = float64(s.W)
	var wf = float64(r.Size.W)

	if align&MarginIsOffset == 0 {
		wf -= float64(marg.W)
		if align&HorCenter != 0 {
			wf -= float64(marg.W)
		}
	}
	//        }
	var ha = float64(s.H)
	var hf = float64(r.Size.H)
	//        if (align & (VertShrink|VertExpand)) {
	if align&MarginIsOffset == 0 {
		hf -= float64(marg.H * 2.0)
	}
	if align&HorExpand != 0 && align&VertExpand != 0 {
		if align&Proportional == 0 {
			wa = wf
			ha = hf
		} else {
			// zlog.Assert(align&HorOut != 0, align) what does this do?
			scalex = wf / wa
			scaley = hf / ha
			if scalex > 1 || scaley > 1 {
				if scalex < scaley {
					wa = wf
					ha *= scalex
				} else {
					ha = hf
					wa *= scaley
				}
			}
		}
	} else if align&Proportional == 0 {
		if align&HorExpand != 0 && wa < wf {
			wa = wf
		} else if align&VertExpand != 0 && ha < hf {
			ha = hf
		}
	}
	if align&HorShrink != 0 && align&VertShrink != 0 && align&Proportional != 0 {
		scalex = wf / wa
		scaley = hf / ha
		if align&HorOut != 0 && align&VertOut != 0 {
			if scalex < 1 || scaley < 1 {
				if scalex > scaley {
					wa = wf
					ha *= scalex
				} else {
					ha = hf
					wa *= scaley
				}
			}
		} else {
			if scalex < 1 || scaley < 1 {
				if scalex < scaley {
					wa = wf
					ha *= scalex
				} else {
					ha = hf
					wa *= scaley
				}
			}
		}
	} else if align&HorShrink != 0 && wa > wf {
		wa = wf
	}
	//  else
	if align&VertShrink != 0 && ha > hf {
		ha = hf
	}
	if maxSize.W != 0 && maxSize.H != 0 { // TODO:  && align&Proportional != 0 {
		s := Size{wa, ha}.ShrunkInto(maxSize)
		wa = s.W
		ha = s.H
	}
	if maxSize.W != 0.0 {
		wa = math.Min(wa, float64(maxSize.W))
	}
	if maxSize.H != 0.0 {
		ha = math.Min(ha, float64(maxSize.H))
	}

	if minSize.W != 0 && minSize.H != 0 && align&Proportional != 0 {
		s := Size{wa, ha}.ExpandedInto(minSize)
		wa = s.W
		ha = s.H
	}
	if minSize.W != 0.0 {
		wa = math.Max(wa, float64(minSize.W))
	}
	if minSize.H != 0.0 {
		ha = math.Max(ha, float64(minSize.H))
	}

	if align&HorOut != 0 {
		if align&Left != 0 {
			x = float64(r.Pos.X - marg.W - s.W)
		} else if align&HorCenter != 0 {
			//                x = float64(Pos.X) - wa / 2.0
			x = float64(r.Pos.X) + (wf-wa)/2.0
		} else {
			x = float64(r.Max().X + marg.W)
		}
	} else {
		if align&Left != 0 {
			x = float64(r.Pos.X + marg.W)
		} else if align&Right != 0 {
			x = float64(r.Max().X) - wa - float64(marg.W)
		} else {
			x = float64(r.Pos.X)
			if align&MarginIsOffset == 0 {
				x += float64(marg.W)
			}
			x = x + (wf-wa)/2.0
			if align&MarginIsOffset != 0 {
				x += float64(marg.W)
			}
		}
	}

	if align&VertOut != 0 {
		if align&Top != 0 {
			y = float64(r.Pos.Y-marg.H) - ha
		} else if align&VertCenter != 0 {
			y = float64(r.Pos.Y) + (hf-ha)/2.0
		} else {
			y = float64(r.Max().Y + marg.H)
		}
	} else {
		if align&Top != 0 {
			y = float64(r.Pos.Y + marg.H)
		} else if align&Bottom != 0 {
			y = float64(r.Max().Y) - ha - float64(marg.H)
		} else {
			y = float64(r.Pos.Y)
			if align&MarginIsOffset == 0 {
				y += float64(marg.H)
			}
			y = y + math.Max(0.0, hf-ha)/2.0
			if align&MarginIsOffset != 0 {
				y += float64(marg.H)
			}
		}
	}
	return Rect{Pos{x, y}, Size{wa, ha}}
}

func (r Rect) MovedInto(rect Rect) (m Rect) {
	m = r
	m.Pos.X = math.Max(r.Pos.X, rect.Pos.X)
	m.Pos.Y = math.Max(r.Pos.Y, rect.Pos.Y)
	min := m.Max().Min(rect.Max())
	m.Pos.Add(min.Minus(m.Max()))
	return
}

/* #kotlin-raw:
   fun copy() : Rect {
       var r = SetRect()
       r.pos = Pos.copy()
       r.size = Size.copy()
       return r
   }
*/

// Dummy function, but if translated(?) to kotlin needed to actually copy rect, not make reference
func (r Rect) Copy() Rect {
	return r
}

func (r Rect) UnionedWith(rect Rect) Rect {
	if !rect.IsNull() {
		if r.IsNull() {
			r.Pos = rect.Pos.Copy()
			r.Size = rect.Size.Copy()
		} else {
			min := r.Min()
			max := r.Max()
			rmin := rect.Min()
			rmax := rect.Max()
			if rmin.X < min.X {
				r.SetMinX(rmin.X)
			}
			if rmin.Y < min.Y {
				r.SetMinY(rmin.Y)
			}
			if rmax.X > max.X {
				r.SetMaxX(rmax.X)
			}
			if rmax.Y > max.Y {
				r.SetMaxY(rmax.Y)
			}
		}
	}
	return r
}

func (r *Rect) UnionWithPos(pos Pos) {
	if r.IsNull() {
		r.Pos = pos
		return
	}
	min := r.Min()
	max := r.Max()
	if pos.X > max.X {
		r.SetMaxX(pos.X)
	}
	if pos.Y > max.Y {
		r.SetMaxY(pos.Y)
	}
	if pos.X < min.X {
		r.SetMinX(pos.X)
	}
	if pos.Y < min.Y {
		r.SetMinY(pos.Y)
	}
}

func (r Rect) Plus(a Rect) Rect     { return RectFromMinMax(r.Pos.Plus(a.Pos), r.Max().Plus(a.Max())) }
func (r Rect) PlusPos(pos Pos) Rect { n := r; n.Pos.Add(pos); return n }
func (r Rect) Minus(a Rect) Rect    { return RectFromMinMax(r.Pos.Minus(a.Pos), r.Max().Minus(a.Max())) }
func (r Rect) DividedBy(a Size) Rect {
	return RectFromMinMax(r.Min().DividedBy(a.Pos()), r.Max().DividedBy(a.Pos()))
}
func (r Rect) TimesD(d float64) Rect {
	return RectFromMinMax(r.Min().TimesD(d), r.Max().TimesD(d))
}

func (r *Rect) Add(a Rect)     { r.SetMin(r.Min().Plus(a.Pos)); r.SetMax(r.Max().Plus(a.Max())) }
func (r *Rect) AddPos(a Pos)   { r.Pos.Add(a) }
func (r *Rect) Subtract(a Pos) { r.Pos.Subtract(a) }

func centerToRect(center Pos, radius float64, radiusy float64) Rect {
	var s = Size{radius, radius}
	if radiusy != 0 {
		s.H = radiusy
	}
	return Rect{center.Minus(s.Pos()), s.TimesD(2.0)}
}

func (r *Rect) ExpandedToInt() Rect {
	var ir Rect
	ir.Pos.X = math.Floor(r.Pos.X)
	ir.Pos.Y = math.Floor(r.Pos.Y)
	ir.Size.W = math.Ceil(r.Size.W)
	ir.Size.H = math.Ceil(r.Size.H)

	return ir
}

// Swapped returns a Rect where the vertical and horizontal components are swapped
func (r Rect) Swapped() Rect {
	return Rect{Pos: r.Pos.Swapped(), Size: r.Size.Swapped()}
}
