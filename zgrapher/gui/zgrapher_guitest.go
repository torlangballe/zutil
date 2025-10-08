//go:build zui && test

package main

import (
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zgrapher"
)

func main() {
	zapp.New()
	zapp.SetUIDefaults(true)
	stack := zcontainer.StackViewVert("stack")
	att := zpresent.AttributesDefault()
	att.MakeFull = true
	label := zlabel.New("Graph Test")
	stack.Add(label, zgeo.TopLeft)

	var job zgrapher.Job
	job.ID = "test1"
	job.PixelHeight = 20
	job.SecondsPerPixel = 5
	job.WindowMinutes = 20
	graph := zgrapher.NewGraphView(job, "grapherTest")
	graph.MinWidth = 500
	graph.ImagePathPrefix = "http://localhost:7776/"
	stack.Add(graph, zgeo.TopLeft)

	zpresent.PresentView(stack, att)
	select {}
}
