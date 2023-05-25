// an UploadView is a gui control that allows the user to get a file to the backend.
// The allow parameter allows it use Drop, URL, SCP or Upload.
// It has an UploadedHandler function to intercept uploaded data or urls, or it defaults to
// Uploading it to the server via POST method.
// For URL and SCP, the file is copied in the backend. For Drop/Upload it is in the post body.
// It has a HandleID used to invoke the correct UploadedHandler and handler in server.
// Set AcceptExtensions to limit draggable/uploadable file types.
// Calling RegisterWidget allows widget:zupload tags in a struct field to create an uploader.
//   the "handleid", "allow" and "ext' tags are used to set fields.
//   use SetWidgeterFileHandler to set a handle for this upload widget.

//go:build zui

package zupload

import (
	"net/url"
	"path"
	"strings"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zui/zwidget"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type UploadView struct {
	zcontainer.StackView
	mainText         *ztext.TextView
	password         *ztext.TextView
	button           *zbutton.Button
	upload           *zbutton.Button
	DropWell         *zwidget.DropWell
	actionMenu       *zmenu.MenuView
	activity         *zwidget.ActivityView
	HandleID         string
	AcceptExtensions []string
	TargetDirectory  string // if empty, a temporary directory is used
	UploadedHandler  func(payload UploadPayload, data []byte)
}

type UploadWidgeter struct{}

var (
	allTypes         = []string{Drop, URL, SCP, Upload}
	widgeterHandlers = map[string]func(up UploadPayload, data []byte){}
	storeKeyPrefix   = "zwidgets.UploadView."
)

func RegisterWidget() {
	zfields.RegisterWidgeter("zupload", UploadWidgeter{})
}

func NewUploadView(storeName string, allow []string) *UploadView {
	v := &UploadView{}
	v.Init(v, storeName, allow)
	return v
}

func (v *UploadView) Init(view zview.View, storeName string, allow []string) {
	v.StackView.Init(v, false, storeName)
	v.SetMinSize(zgeo.Size{0, 32}) // avoids
	var items zdict.Items
	for _, a := range allTypes {
		if len(allow) == 0 || zstr.StringsContain(allow, a) {
			items = append(items, zdict.Item{a, a})
		}
	}
	actionKey := storeKeyPrefix + "Action"
	set, _ := zkeyvalue.DefaultStore.GetString(actionKey)
	if set == "" {
		set = Upload
	}
	v.actionMenu = zmenu.NewView("allow", items, set)
	v.actionMenu.SetSelectedHandler(func() {
		set := v.actionMenu.CurrentValue().(string)
		zkeyvalue.DefaultStore.SetString(set, actionKey, true)
		v.updateWidgets()
	})
	v.Add(v.actionMenu, zgeo.CenterLeft)

	textKey := storeKeyPrefix + "Text"
	text, _ := zkeyvalue.DefaultStore.GetString(textKey)
	tstyle := ztext.Style{KeyboardType: zkeyboard.TypeURL}
	v.mainText = ztext.NewView(text, tstyle, 30, 1)
	v.mainText.SetChangedHandler(func() {
		zkeyvalue.DefaultStore.SetString(v.mainText.Text(), textKey, true)
		v.updateWidgets()
	})
	v.mainText.UpdateSecs = 0
	v.Add(v.mainText, zgeo.CenterLeft|zgeo.HorExpand)

	passKey := storeKeyPrefix + "Password"
	pass, _ := zkeyvalue.DefaultStore.GetString(passKey)
	tstyle = ztext.Style{KeyboardType: zkeyboard.TypePassword}
	v.password = ztext.NewView(pass, tstyle, 10, 1)
	v.password.SetPlaceholder("password")
	v.password.SetChangedHandler(func() {
		zkeyvalue.DefaultStore.SetString(v.password.Text(), passKey, true)
		v.updateWidgets()
	})
	v.password.UpdateSecs = 0
	v.Add(v.password, zgeo.CenterLeft)

	v.button = zbutton.New("")
	v.Add(v.button, zgeo.CenterLeft)
	v.button.SetPressedHandler(v.buttonPressed)

	v.DropWell = zwidget.NewDropWell("", zgeo.Size{10, 20})
	v.Add(v.DropWell, zgeo.CenterLeft|zgeo.Expand)
	v.DropWell.HandleDroppedFile = v.handleGivenFile
	v.DropWell.HandleDropPreflight = v.checkExtensions

	v.activity = zwidget.NewActivityView(zgeo.SizeBoth(16))
	v.Add(v.activity, zgeo.CenterLeft)
}

func (v *UploadView) ReadyToShow(beforeWin bool) {
	if beforeWin {
		v.updateWidgets()
	}
}

func (v *UploadView) checkExtensions(name string) bool {
	if len(v.AcceptExtensions) > 0 {
		ext := path.Ext(name)
		if !zstr.StringsContain(v.AcceptExtensions, ext) {
			zalert.ShowError(nil, "Incorrect file extension:", ext, v.AcceptExtensions)
			return true
		}
	}
	v.activity.Start()
	return false
}

func (v *UploadView) callUploadHandler(up UploadPayload, data []byte) {
	v.activity.Start()
	v.UploadedHandler(up, data)
	v.activity.Stop()
}

func (v *UploadView) handleGivenFile(data []byte, name string) {
	// zlog.Info("handleGivenFile:", name)
	var up UploadPayload
	up.HandleID = v.HandleID
	up.Name = name
	up.Type = v.actionMenu.CurrentValue().(string)
	go v.callUploadHandler(up, data)
}

func (v *UploadView) buttonPressed() {
	var up UploadPayload
	up.HandleID = v.HandleID
	up.Type = v.actionMenu.CurrentValue().(string)
	up.Text = v.mainText.Text()
	switch up.Type {
	case URL:
	case SCP:
		up.Password = v.password.Text()
	}
	go v.callUploadHandler(up, nil)
}

func (v *UploadView) updateWidgets() {
	var ctext, cpass, cbutton, cdrop, cactivity bool
	var tbutton string
	var ptext string
	busable := true
	action := v.actionMenu.CurrentValue().(string)
	switch action {
	case Drop:
		ctext, cpass, cbutton, cdrop, cactivity = true, true, true, false, false
	case URL:
		ctext, cpass, cbutton, cdrop, cactivity = false, true, false, true, false
		tbutton = "copy"
		ptext = "URL to copy from"
		u, _ := url.Parse(v.mainText.Text())
		busable = (u != nil && u.Host != "" && len(u.Path) >= 2 && u.Scheme != "")
	case SCP:
		ctext, cpass, cbutton, cdrop, cactivity = false, false, false, true, false
		tbutton = "copy"
		ptext = "user@address:path to copy from"
		str := v.mainText.Text()
		ai := strings.Index(str, "@")
		ci := strings.Index(str, ":")
		plen := len(v.password.Text())
		busable = (len(str) > 10 && ai > 1 && ci > ai && plen >= 2)
	case Upload:
		if v.upload == nil {
			v.addUploadButton()
		}
		ctext, cpass, cbutton, cdrop, cactivity = true, true, true, true, false
	}
	if action != Upload && v.upload != nil {
		v.RemoveChild(v.upload)
		v.upload = nil
	}
	v.button.SetUsable(busable)
	v.button.SetText(tbutton)
	v.mainText.SetPlaceholder(ptext)
	v.CollapseChild(v.mainText, ctext, false)
	v.CollapseChild(v.password, cpass, false)
	v.CollapseChild(v.button, cbutton, false)
	v.CollapseChild(v.DropWell, cdrop, false)
	v.CollapseChild(v.activity, cactivity, true)
}

func (v *UploadView) addUploadButton() {
	v.upload = zbutton.New("choose file")
	v.upload.SetMinWidth(100)
	len := v.CountChildren()
	v.AddAdvanced(v.upload, zgeo.CenterLeft, zgeo.Size{}, zgeo.Size{}, len-1, false)
	v.upload.SetUploader(v.handleGivenFile, v.checkExtensions, nil)
}

func (a UploadWidgeter) Create(f *zfields.Field) zview.View {
	min := f.MinWidth
	if min == 0 {
		min = 100
	}
	var allow []string
	sallow := f.CustomFields["allow"]
	for _, a := range strings.Split(sallow, "|") {
		if zstr.StringsContain(allTypes, a) {
			allow = append(allow, a)
		}
	}
	// zlog.Info("UploadWidgeter:", allows, allowMask)
	uploader := NewUploadView(f.ValueStoreKey, allow)
	uploader.DropWell.SetPlaceholder(f.Placeholder)
	sext := f.CustomFields["ext"]
	if sext != "" {
		uploader.AcceptExtensions = strings.Split(sext, "|")
	}
	uploader.HandleID = f.CustomFields["handleid"]
	zlog.Assert(len(uploader.HandleID) > 0)
	zlog.Assert(uploader.HandleID != "")
	handler := widgeterHandlers[uploader.HandleID]
	if handler != nil {
		uploader.UploadedHandler = handler
	} else {
		uploader.UploadedHandler = CallHTTUpload
	}
	if f.Styling.FGColor.Valid {
		col := f.Styling.FGColor
		if col.Valid {
			uploader.SetColor(col)
		}
	}
	return uploader
}

func CallHTTUpload(up UploadPayload, data []byte) {
	var result zdict.Dict
	params := zhttp.MakeParameters()
	args := map[string]string{
		"id":   up.HandleID,
		"name": up.Name,
		"text": up.Text,
		"type": up.Type,
	}
	params.Headers["X-Password"] = up.Password
	params.TimeoutSecs = 120
	// params.PrintBody = true
	surl := zapp.DownloadPathPrefix + "zupload"
	surl, _ = zhttp.MakeURLWithArgs(surl, args)
	_, err := zhttp.Post(surl, params, data, &result)
	if err != nil {
		zalert.ShowError(err)
	}
	if result != nil {
		str := result["message"]
		if str != "" {
			zalert.Show(str)
		}
	}
	// zlog.Info("CallHTTUpload Done", err, storedURL)
}

func (a UploadWidgeter) IsStatic() bool {
	return true
}

func (a UploadWidgeter) SetValue(view zview.View, val any) {
}

func SetWidgeterFileHandler(id string, handler func(up UploadPayload, data []byte)) {
	widgeterHandlers[id] = handler
}
