//go:build zui

package zdebug

import (
	"bytes"
	"runtime/pprof"

	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
)

type DebugView struct {
	zcontainer.StackView
}

func doProfiling(ptype string) []byte {
	out := bytes.NewBuffer([]byte{})
	pprof.WriteHeapProfile(out)
	defer pprof.StopCPUProfile()
	return out.Bytes()
}

func addProfileRow(in *zcontainer.StackView, name, ptype string) *zcontainer.StackView {
	if ptype == "" {
		ptype = name
	}
	v := zcontainer.StackViewHor(name + "-stack")
	v.SetMarginS(zgeo.Size{10, 10})
	button := zbutton.New(name)
	button.SetPressedHandler(func() {
		data := doProfiling(ptype)
		uri := zhttp.MakeDataURL(data, "application/octet-stream")
		zview.DownloadURI(uri, name)
	})
	v.Add(button, zgeo.CenterLeft)

	down := "<download-folder>"
	if zdevice.WasmBrowser() == zdevice.Safari {
		down = "~/Downloads"
	}
	label := zlabel.New("go tool pprof -web " + down + "/" + name + ".gz")
	v.Add(label, zgeo.CenterLeft)

	in.Add(v, zgeo.CenterLeft)
	return v
}

func NewDebugView() *DebugView {
	v := &DebugView{}
	v.Init(v, true, "debug-view")
	addProfileRow(&v.StackView, "heap", "")
	v.SetMinSize(zgeo.SizeF(400, 400))
	return v
}

func PresentDebugView() {
	v := NewDebugView()
	att := zpresent.AttributesNew()
	att.Modal = true
	att.ModalCloseOnOutsidePress = true
	zpresent.PresentTitledView(v, "Debug", att, nil, nil)
}
