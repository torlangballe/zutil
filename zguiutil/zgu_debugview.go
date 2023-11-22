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
	button := zbutton.New("download " + name)
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

func addDownloadRow(in *zcontainer.StackView, ip, name, ptype string) {
	button, _ := addRow(in, name, ptype)
	surl := zapp.URLStub()
	if ip != "" {
		u := zapp.URL()
		u.Path = ""
		u.RawQuery = ""
		u.Host = ip
		surl = u.String()
	}
	surl += zrest.AppURLPrefix + "debug/pprofile/" + name // must be here and not in closure below!
	button.SetToolTip(surl)
	button.SetPressedHandler(func() {
		zview.DownloadURI(surl, name)
	})
}

func makeFrame(in *zcontainer.StackView, name string) *zcontainer.StackView {
	frame := zcontainer.StackViewVert("other")
	MakeStackATitledFrame(frame, name, false, DefaultFrameStyling, DefaultFrameTitleStyling)
	in.Add(frame, zgeo.CenterLeft|zgeo.HorExpand)
	return frame
}

func NewDebugView(urlStub string, otherIPs map[string]string) *DebugView {
	v := &DebugView{}
	v.SetMarginS(zgeo.Size{10, 10})
	v.Init(v, true, "debug-view")
	frame := makeFrame(&v.StackView, "gui")
	addGUIProfileRow(frame, "gui-heap", "heap")
	frame = makeFrame(&v.StackView, "manager")
	for _, name := range zdebug.AllProfileTypes {
		addDownloadRow(frame, "", name, name)
	}
	for name, ip := range otherIPs {
		frame = makeFrame(&v.StackView, name)
		addDownloadRow(frame, ip, "heap", "heap")
	}
	frame = makeFrame(&v.StackView, "Enable GUI named log sections")
	zlog.EnablerList.ForEach(func(name string, e *zlog.Enabler) bool {
		check, _, stack := zcheckbox.NewWithLabel(false, name, name+".zlog.Enabler")
		frame.Add(stack, zgeo.CenterLeft)
		check.SetValueHandler(func() {
			*e = zlog.Enabler(check.On())
		})
		return true
	})
	v.SetMinSize(zgeo.SizeF(400, 400))
	return v
}

func PresentDebugView(urlStub string, otherIPs map[string]string) {
	v := NewDebugView(urlStub, otherIPs)
	att := zpresent.AttributesNew()
	att.Modal = true
	att.ModalCloseOnOutsidePress = true
	zpresent.PresentTitledView(v, "Debug", att, nil, nil)
}
