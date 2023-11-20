//go:build zui

package zguiutil

import (
	"bytes"
	"runtime/pprof"

	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
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

func addRow(in *zcontainer.StackView, name, ptype string) (*zbutton.Button, *zlabel.Label) {
	v := zcontainer.StackViewHor(name + "-stack")
	button := zbutton.New(name)
	v.Add(button, zgeo.CenterLeft)
	down := "<download-folder>"
	if zdevice.WasmBrowser() == zdevice.Safari {
		down = "~/Downloads"
	}
	then := zlabel.New("then")
	then.SetFont(zgeo.FontDefault().NewWithStyle(zgeo.FontStyleBold))
	v.Add(then, zgeo.CenterLeft)
	label := zlabel.New("go tool pprof -web " + down + "/" + ptype + ".gz")
	ztext.MakeViewPressToClipboard(label)
	v.Add(label, zgeo.CenterLeft)
	in.Add(v, zgeo.CenterLeft|zgeo.HorExpand)
	return button, label
}

func addGUIProfileRow(in *zcontainer.StackView, name, ptype string) {
	button, _ := addRow(in, name, ptype)
	if ptype == "" {
		ptype = name
	}
	button.SetPressedHandler(func() {
		data := doProfiling(ptype)
		uri := zhttp.MakeDataURL(data, "application/octet-stream")
		zview.DownloadURI(uri, name)
	})
}

func addDownloadRow(in *zcontainer.StackView, name, ptype string) {
	button, _ := addRow(in, name, ptype)
	surl := zapp.URLStub() + zrest.AppURLPrefix + "debug/pprofile/" + name // must be here and not in closure below!
	button.SetPressedHandler(func() {
		zview.DownloadURI(surl, name)
	})
}

func NewDebugView(urlStub string) *DebugView {
	v := &DebugView{}
	v.SetMarginS(zgeo.Size{10, 10})
	v.Init(v, true, "debug-view")
	addGUIProfileRow(&v.StackView, "gui-heap", "heap")
	for _, name := range zdebug.AllProfileTypes {
		addDownloadRow(&v.StackView, name, name)
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
