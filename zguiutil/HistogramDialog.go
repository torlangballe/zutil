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

func PopupHistogramDialog(h *zhistogram.Histogram, title, name string, criticalVal float64, att *zpresent.Attributes, transformClass func(bar float64) string) {
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
	builder.AddLabelsRowToVertStack(grid, blue, zgeo.FontStyleBold, zstyle.Start, name, "Percent of Total", "Count")
	barVal := h.Range.Min
	for _, c := range h.Classes {
		sclass := zwords.NiceFloat(barVal, 0) + "-" + zwords.NiceFloat(barVal+h.Step, 0)
		scount := fmt.Sprint(c)
		spercent := fmt.Sprint(h.CountAsPercent(c), "%")
		if c == 0 {
			spercent = ""
			scount = " "
		}
		var param any
		param = zstyle.NoOp
		if criticalVal != 0 && barVal >= criticalVal {
			param = zgeo.ColorRed
		}
		builder.AddLabelsRowToVertStack(grid, param, zstyle.Start, zgeo.FontStyleBold, sclass, spercent, scount)
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
