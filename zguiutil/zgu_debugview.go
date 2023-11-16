//go:build zui

package zguiutil

import (
	"bytes"
	"runtime/pprof"

	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
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

func NewDebugView(urlStub string) *DebugView {
	v := &DebugView{}
	v.SetMarginS(zgeo.Size{10, 10})
	v.Init(v, true, "debug-view")
	addProfileRow(&v.StackView, "heap", "")
	for _, p := range zdebug.GetProfileCommandLineGetters(urlStub) {
		label := zlabel.New(p)
		v.Add(label, zgeo.CenterLeft)
	}
	zlog.EnablerList.ForEach(func(name string, e *zlog.Enabler) bool {
		check, _, stack := zcheckbox.NewWithLabel(false, name, name+".zlog.Enabler")
		v.Add(stack, zgeo.CenterLeft)
		check.SetValueHandler(func() {
			*e = zlog.Enabler(check.On())
		})
		return true
	})
	v.SetMinSize(zgeo.SizeF(400, 400))
	return v
}

func PresentDebugView(urlStub string) {
	v := NewDebugView(urlStub)
	att := zpresent.AttributesNew()
	att.Modal = true
	att.ModalCloseOnOutsidePress = true
	zpresent.PresentTitledView(v, "Debug", att, nil, nil)
}
