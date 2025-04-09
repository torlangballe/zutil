//go:build zui

package zguiutil

import (
	"fmt"

	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmath/zhistogram"
	"github.com/torlangballe/zutil/zwords"
)

func PopupHistogramDialog(h *zhistogram.Histogram, title, name string, criticalVal float64, att *zpresent.Attributes, transformName func(n string) (string, zgeo.Color)) {
	v := zcontainer.StackViewVert("histogram")
	v.SetMarginS(zgeo.SizeD(12, 12))
	grid := zcontainer.StackViewVert("grid")
	grid.GridVerticalSpace = 5
	grid.SetSpacing(20)
	v.Add(grid, zgeo.TopLeft|zgeo.Expand)

	if name == "" {
		name = "Range"
	}

	builder := zlabel.NewStyledTextBuilder()
	builder.Default.Gap = 16
	blue := zgeo.ColorNew(0.2, 0.2, 1, 1)
	builder.AddLabelsRowToVertStack(grid, blue, zgeo.FontStyleBold, zstyle.Start, name, "% of Total", "Count")
	barVal := h.Range.Min
	for i, c := range h.Classes {
		var sclass string
		var col zgeo.Color
		if h.IsNames {
			sclass = h.Classes[i].Label
			before := sclass
			if transformName != nil {
				sclass, col = transformName(sclass)
			}
			if sclass == "" {
				sclass = fmt.Sprint("none:", before)
			}
		} else {
			sa := zwords.NiceFloat(barVal, 0)
			sb := zwords.NiceFloat(barVal+h.Step, 0)
			sclass = sa + "-" + sb
		}
		spercent := fmt.Sprint(h.CountAsPercent(c.Count), "%")
		scount := fmt.Sprint(c.Count)
		if c.Count == 0 {
			spercent = ""
			scount = " "
		}
		textCol := col.ContrastingGray()
		if criticalVal != 0 && barVal >= criticalVal {
			textCol = zgeo.ColorRed
		}
		h1 := builder.AddLabelsRowToVertStack(grid, textCol, zstyle.Start, zgeo.FontStyleBold, sclass, spercent, scount)
		if col.Valid && len(h.Classes) > 1 {
			h1.SetBGColor(col)
			h1.SetCorner(2)
			h1.SetMarginS(zgeo.SizeD(2, 0))
		}
		barVal += h.Step
		barVal = zfloat.KeepFractionDigits(barVal, 7)
	}
	if h.OutlierBelow != 0 {
		builder.AddLabelsRowToVertStack(grid, "Outliers Below", fmt.Sprint(h.OutlierBelow), fmt.Sprint(h.CountAsPercent(h.OutlierBelow), "%"))
	}
	if h.OutlierAbove != 0 {
		builder.AddLabelsRowToVertStack(grid, "Outliers Above", fmt.Sprint(h.OutlierAbove), fmt.Sprint(h.CountAsPercent(h.OutlierAbove), "%"))
	}
	if att == nil {
		att = &zpresent.Attributes{}
		*att = zpresent.ModalConfirmAttributes
	}
	att.ModalCloseOnOutsidePress = true
	att.ModalDismissOnEscapeKey = true
	att.ModalDimBackground = false
	att.ModalStrokeWidth = 1
	att.ModalStrokeColor = zgeo.ColorBlack
	att.ModalCorner = 16
	zpresent.PresentTitledView(v, title, *att, nil, nil)
}
