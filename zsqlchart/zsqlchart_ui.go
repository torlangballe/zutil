// +build zui

package zsqlchart

import (
	"github.com/torlangballe/zfields"
	"github.com/torlangballe/zui"
	"github.com/torlangballe/zutil/zgeo"
)

type RowColor struct {
	ResultRow string
	Color     zgeo.Color
}
type ChartInfo struct {
	Name   string
	Inputs struct {
		Arguments []interface{} `zui:"group"`
		Results   []RowColor    `zui:"-"`
	} `zui:"-"`
	YAxisName string
	Query     string `zui:"-"`
}

type ChartsView struct {
	zui.ScrollView
	info           ChartInfo
	chartsInfoView *zfields.FieldView
	Renderer       *zui.GraphView
}

func ChartsViewNew(storeKey string) *ChartsView {
	v := &ChartsView{}
	v.Init(v, "charts")

	zui.DefaultLocalKeyValueStore.GetObject(storeKey, &v.info)
	stack := zui.StackViewVert("stack")
	stack.SetMargin(zgeo.RectFromXY2(10, 10, -10, -10))

	v.chartsInfoView = zfields.FieldViewNew("auth", &v.info, 0)
	v.chartsInfoView.SetSpacing(12)
	stack.AddChild(v.chartsInfoView, -1)

	v.chartsInfoView.Build(true)

	v.Renderer = zui.GraphViewNew(zgeo.Size{200, 200})
	stack.Add(v.Renderer, zgeo.TopLeft|zgeo.Expand)

	help := zui.DocumentationIconViewNew("auth.md")
	stack.AddAdvanced(help, zgeo.TopRight, zgeo.Size{0, -2}, zgeo.Size{}, -1, true)

	run := zui.ImageButtonViewNewSimple("Run", "green")
	stack.Add(run, zgeo.BottomRight)
	return v
}
