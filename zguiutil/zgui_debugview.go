//go:build zui

package zguiutil

import (
	"bytes"
	"runtime/pprof"

	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zclipboard"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zui/zwidgets"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zgraphana"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/ztimer"
)

type DebugView struct {
	zcontainer.StackView
}

func doProfiling(ptype string) []byte {
	out := bytes.NewBuffer([]byte{})
	pprof.Lookup(ptype).WriteTo(out, 0)
	o := out.Bytes()
	if ptype == "profile" {
		pprof.StopCPUProfile()
	}
	return o
}

func addRow(in *zcontainer.StackView, name, ptype string) (*zbutton.Button, *zlabel.Label, *zwidgets.ActivityView) {
	var activity *zwidgets.ActivityView
	v := zcontainer.StackViewHor(name + "-stack")
	button := zbutton.New("download " + name)
	v.Add(button, zgeo.CenterLeft)
	down := "<download-folder>"
	if zdevice.WasmBrowser() == zdevice.Safari {
		down = "~/Downloads"
	}
	then := zlabel.New("then")
	then.SetFont(zgeo.FontDefault(0).NewWithStyle(zgeo.FontStyleBold))
	v.Add(then, zgeo.CenterLeft)
	label := zlabel.New("go tool pprof -web " + down + "/" + name)
	label.SetPressWithModifierToClipboard(zkeyboard.ModifierNone)
	v.Add(label, zgeo.CenterLeft)
	in.Add(v, zgeo.CenterLeft|zgeo.HorExpand)

	if name == "cpu" {
		activity = zwidgets.NewActivityView(zgeo.SizeBoth(14), zgeo.ColorBlack)
		v.Add(activity, zgeo.CenterLeft)
	}
	return button, label, activity
}

func addGUIProfileRow(in *zcontainer.StackView, name, ptype string) {
	button, _, _ := addRow(in, name, ptype)
	if ptype == "" {
		ptype = name
	}
	button.SetPressedHandler("", zkeyboard.ModifierNone, func() {
		data := doProfiling(ptype)
		uri := zhttp.MakeDataURL(data, "application/octet-stream")
		zview.DownloadURI(uri, name)
	})
}

func addDownloadRow(in *zcontainer.StackView, ip, name, ptype string) {
	button, _, activity := addRow(in, name, ptype)
	surl := zapp.URLStub()
	if ip != "" {
		u := zapp.URL() // for scheme+port
		u.Path = ""
		u.RawQuery = ""
		u.Host = ip
		surl = u.String()
	}
	surl += zrest.AppURLPrefix + zdebug.ProfilingURLPrefix + name // must be here and not in closure below!
	button.SetToolTip(surl)
	button.SetPressedHandler("", zkeyboard.ModifierAlt, func() {
		str := "curl " + surl + " > " + name + " && go tool pprof -web " + name
		zclipboard.SetString(str)
		return
	})
	button.SetPressedHandler("", zkeyboard.ModifierNone, func() {
		if activity != nil {
			activity.Start()
			ztimer.StartIn(10, activity.Stop)
		}
		go zview.DownloadURI(surl, name)
	})
}

func makeFrame(in *zcontainer.StackView, name string) (frame, header *zcontainer.StackView) {
	frame = zcontainer.StackViewVert("other")
	header = MakeStackATitledFrame(frame, name, false, DefaultFrameStyling, DefaultFrameTitleStyling)
	in.Add(frame, zgeo.CenterLeft|zgeo.HorExpand)
	return frame, header
}

func NewDebugView(urlStub string, otherIPs map[string]string, serverName string, prometheusPortOpt *zkeyvalue.Option[int]) *DebugView {
	v := &DebugView{}
	v.SetMarginS(zgeo.SizeD(10, 10))
	v.Init(v, true, "debug-view")
	frame, _ := makeFrame(&v.StackView, "gui")
	addGUIProfileRow(frame, "gui-heap", "heap")
	frame, _ = makeFrame(&v.StackView, "manager")
	for _, name := range zdebug.AllProfileTypes {
		addDownloadRow(frame, "", name, name)
	}
	for device, ip := range otherIPs {
		frame, _ = makeFrame(&v.StackView, device)
		for _, name := range []string{"heap", "cpu"} {
			addDownloadRow(frame, ip, name, name)
		}
	}
	frame, _ = makeFrame(&v.StackView, "Enable GUI named log sections")
	zlog.EnablerList.Range(func(k, v any) bool {
		name := k.(string)
		e := v.(*zlog.Enabler)
		check, _, stack := zcheckbox.NewWithLabel(false, name, name+".zlog.Enabler")
		frame.Add(stack, zgeo.CenterLeft)
		check.SetValueHandler("", func(edited bool) {
			*e = zlog.Enabler(check.On())
		})
		return true
	})
	var header *zcontainer.StackView
	frame, header = makeFrame(&v.StackView, "Telemetry")
	grid := zcontainer.GridViewNew("options", 2)
	grid.Spacing.H = 0
	frame.Add(grid, zgeo.TopLeft|zgeo.Expand)
	AddKVOptionToGrid(grid, zgraphana.APIKey)
	AddKVOptionToGrid(grid, zgraphana.URLPrefix)
	AddKVOptionToGrid(grid, zgraphana.DashboardUID)
	AddKVOptionToGrid(grid, prometheusPortOpt)
	link := zlabel.NewLink("dashboard", "", true)
	header.Add(link, zgeo.CenterRight)
	timer := ztimer.RepeatForeverNow(1, func() {
		pref := zgraphana.URLPrefix.Get()
		uid := zgraphana.DashboardUID.Get()
		var surl string
		var args = map[string]string{
			"theme": "dark",
		}
		if pref != "" && uid != "" {
			surl = zfile.JoinPathParts(pref, "D", uid)
			if serverName != "" {
				args["var-job"] = serverName
			}
		}
		surl, _ = zhttp.MakeURLWithArgs(surl, args)
		link.SetURL(surl, true)
	})
	v.AddOnRemoveFunc(timer.Stop)
	v.SetMinSize(zgeo.SizeF(400, 400))
	return v
}

func PresentDebugView(urlStub string, otherIPs map[string]string, serverName string, prometheusPortOpt *zkeyvalue.Option[int]) {
	v := NewDebugView(urlStub, otherIPs, serverName, prometheusPortOpt)
	att := zpresent.AttributesNew()
	att.Modal = true
	att.ModalCloseOnOutsidePress = true
	zpresent.PresentTitledView(v, "Debug", att, nil, nil)
}
