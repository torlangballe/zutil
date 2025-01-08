//go:build zui

package zguiutil

import (
	"strings"

	"github.com/torlangballe/zui/zcanvas"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/ztextinfo"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/ztimer"
)

func NewBar(title string) (*zcontainer.StackView, *zlabel.Label) {
	var label *zlabel.Label
	bar := zcontainer.StackViewHor("bar")
	bar.SetMargin(zgeo.RectFromXY2(6, 0, -6, -3))

	if title != "" {
		label = zlabel.New(title)
		label.SetObjectName("title")
		label.SetFont(zgeo.FontNew("Arial", 18, zgeo.FontStyleNormal))
		label.SetColor(zstyle.DefaultFGColor())
		label.SetMaxWidth(500)
		label.SetMaxLines(1)
		label.SetLongPressedHandler("", zkeyboard.ModifierNone, func() {
			zdebug.PrintAllGoroutines()
			ztimer.DumpRepeaters()
		})
		bar.Add(label, zgeo.Left|zgeo.VertCenter|zgeo.HorExpand)
	}
	bar.SetDrawHandler(func(rect zgeo.Rect, canvas *zcanvas.Canvas, view zview.View) {
		colors := []zgeo.Color{
			zstyle.Col(zgeo.ColorNew(0.85, 0.88, 0.91, 1), zgeo.ColorNew(0.15, 0.18, 0.21, 1)),
			zstyle.Col(zgeo.ColorNew(0.69, 0.72, 0.76, 1), zgeo.ColorNew(0.29, 0.32, 0.36, 1)),
		}
		path := zgeo.PathNewRect(rect, zgeo.SizeNull)
		canvas.DrawGradient(path, colors, rect.Min(), rect.BottomLeft(), nil)
	})
	return bar, label
}

func makeLabelizeLabel(text string, postfix string, talign zgeo.Alignment) *zlabel.Label {
	label := zlabel.New(text)
	label.SetTextAlignment(talign)
	if postfix == "" {
		postfix = "desc"
	}
	label.SetObjectName("$labelize.label." + postfix)
	return label
}

func Labelize(view zview.View, slabel string, minLabelWidth float64, alignment zgeo.Alignment, desc string) (label *zlabel.Label, stack *zcontainer.StackView, viewCell *zcontainer.Cell, descLabel *zlabel.Label) {
	font := zgeo.FontNice(zgeo.FontDefaultSize, zgeo.FontStyleBold)
	to, _ := view.(ztextinfo.Owner)
	if to != nil {
		ti := to.GetTextInfo()
		font = ti.Font
		zfloat.Maximize(&font.Size, zgeo.FontDefaultSize)
		font.Style = zgeo.FontStyleBold
	}
	title := slabel
	checkBox, isCheck := view.(*zcheckbox.CheckBox)
	if checkBox != nil && alignment&zgeo.Right != 0 {
		title = ""
		_, cstack := zcheckbox.Labelize(checkBox, slabel)
		view = cstack
		alignment = alignment.FlippedHorizontal()
	}
	label = makeLabelizeLabel(title, slabel, zgeo.Right)
	label.SetFont(font)
	label.SetColor(zstyle.DefaultFGColor().WithOpacity(0.8))
	stack = zcontainer.StackViewHor("$labelize.stack." + slabel) // give it special name so not easy to mis-search for in recursive search
	stack.SetChildrenAboveParent(true)
	stack.SetSpacing(30)
	stack.SetMargin(zgeo.RectFromXY2(0, 0, -3, 0))
	a := zgeo.VertCenter
	if alignment&zgeo.Vertical != 0 {
		a = (a & ^zgeo.Vertical) | (alignment & zgeo.Vertical)
	}
	if a&zgeo.Horizontal == 0 {
		a |= zgeo.Left
	}
	// zlog.Info("zgui.Labelzie:", view.Native().Hierarchy(), a, alignment)

	cell := stack.Add(label, a)
	if minLabelWidth != 0 {
		cell.MinSize.W = minLabelWidth
	}
	marg := zgeo.SizeNull
	if isCheck {
		// marg.W = -6 // in html cell has a box around it of 20 pixels
	}
	viewCell = stack.Add(view, alignment, marg)

	if desc != "" {
		descLabel = makeLabelizeLabel(desc, slabel+".desc", zgeo.Left)
		font.Style = zgeo.FontStyleNormal
		lines := strings.Count(desc, "\n") + 1
		descLabel.SetMaxLines(lines)
		descLabel.SetFont(font)
		descLabel.SetColor(zstyle.DefaultFGColor().Mixed(zgeo.ColorBlue, 0.3))
		stack.Add(descLabel, zgeo.CenterLeft)
		viewCell = &stack.Cells[len(stack.Cells)-2] // we need to re-get the cell in case adding desc made a new slice
	}
	return label, stack, viewCell, descLabel
}

var DefaultFrameStyling = zstyle.Styling{
	StrokeWidth:   2,
	StrokeColor:   zstyle.DefaultFGColor().WithOpacity(0.5),
	Corner:        5,
	StrokeIsInset: zbool.True,
	Margin:        zgeo.RectFromXY2(8, 13, -8, -8),
}

var DefaultFrameTitleStyling = zstyle.Styling{
	FGColor: zstyle.DefaultFGColor().WithOpacity(0.7),
	Font:    *zgeo.FontNice(zgeo.FontDefaultSize, zgeo.FontStyleBold),
}

func MakeStackATitledFrame(stack *zcontainer.StackView, title string, titleOnFrame bool, styling, titleStyling zstyle.Styling) (header *zcontainer.StackView) {
	s := DefaultFrameStyling.MergeWith(styling)
	fs := s
	fs.Font = zgeo.Font{}
	stack.SetStyling(fs)
	if title != "" {
		header = zcontainer.StackViewHor("header")
		header.SetSpacing(2)
		h := -8.0
		if titleOnFrame {
			h = -(s.Margin.Min().Y + zgeo.FontDefaultSize - 4)
			header.SetCorner(4)
			header.SetBGColor(zgeo.ColorWhite)
		}
		stack.AddAdvanced(header, zgeo.TopLeft|zgeo.HorExpand, zgeo.RectFromXY2(0, h, 0, 0), zgeo.SizeNull, 0, false).NotInGrid = true
		label := zlabel.New(title)
		label.SetObjectName("title")
		label.SetMaxWidth(400)
		label.SetWrap(ztextinfo.WrapTailTruncate)
		label.SetPressWithModifierToClipboard(zkeyboard.ModifierAlt)
		ts := DefaultFrameTitleStyling.MergeWith(titleStyling)
		label.SetStyling(ts)
		header.Add(label, zgeo.CenterLeft|zgeo.HorExpand, zgeo.SizeNull)
	}
	return header
}

func AddLabeledViewToGrid(grid *zcontainer.GridView, title string, view zview.View) {
	label := zlabel.New(title)
	grid.Add(label, zgeo.CenterRight, zgeo.SizeNull)

	grid.Add(view, zgeo.CenterLeft|zgeo.HorExpand, zgeo.SizeNull)
}

// CreateLockIconForView creates a lock icon label which enables/disables view.
// It starts disabled if not empty, and handles changes to views emptiness.
func CreateLockIconForView(view zview.View) zview.View {
	label := zlabel.New("🔒")
	clear := true
	to, _ := view.(ztext.TextOwner)
	if to != nil && to.Text() == "" {
		clear = false
	}
	if clear {
		view.Native().SetUsable(false)
	}
	label.SetPressedHandler("$press.lock", zkeyboard.ModifierNone, func() {
		u := view.Native().IsUsable()
		view.Native().SetUsable(!u)
	})
	vh, _ := view.(zview.ValueHandler)
	if vh != nil {
		vh.SetValueHandler("zguiutil.Lock", func(edited bool) {
			to, _ := view.(ztext.TextOwner)
			if edited || to != nil && to.Text() == "" {
				return
			}
			view.Native().SetUsable(false)
		})
	}
	return label
}
