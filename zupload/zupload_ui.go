// an UploadView is a gui control that allows the user to get a file to the backend.
// The allow parameter allows it use Drop, URL, SCP or Upload.
// It has an FileReadyToSendHandler function to intercept data in gui from upload/drag, or it defaults to
// Uploading it to the server via POST method.
// For URL and SCP, the file is copied in the backend. For Drop/Upload it is in the post body.
// It has a HandleID used to invoke the correct method set by RegisterUploadHandler in the server.
// Set AcceptExtensions to limit draggable/uploadable file types.
// Calling RegisterWidget allows widget:zupload tags in a struct field to create an uploader.
//   the "handleid", "allow" and "ext' tags are used to set fields.
//   use SetWidgeterFileHandler to set a handle for this upload widget.

//go:build zui

package zupload

import (
	"errors"
	"net/url"
	"path"
	"strings"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zui/zwidgets"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zstr"
)

type UploadView struct {
	zcontainer.StackView
	HandleID                    string
	AcceptExtensions            []string
	TargetDirectory             string // if empty, a temporary directory is used
	FileReadyToSendHandler      func(payload UploadPayload, data []byte)
	FileUploadedToServerHandler func(result zdict.Dict, err error)
	mainText                    *ztext.TextView
	password                    *ztext.TextView
	button                      *zbutton.Button
	selectButton                *zbutton.Button
	DropWell                    *zwidgets.DropWell
	actionMenu                  *zmenu.MenuView
	activity                    *zwidgets.ActivityView
}

var (
	allTypes       = []string{Drop, URL, SCP, Select}
	storeKeyPrefix = "zwidgets.UploadView."
	AuthTokenFunc  func() string // AuthTokenFunc is set in ui and server, and sent from ui in CallHTTUpload, and checked in server's handleUpload
)

func Init(authTokenGetter func() string) {
	AuthTokenFunc = authTokenGetter
}

func NewUploadView(storeName string, allow []string, storeKey string) *UploadView {
	v := &UploadView{}
	v.Init(v, storeName, allow, storeKey)
	return v
}

func (v *UploadView) Init(view zview.View, storeName string, allowTypes []string, storeKey string) {
	v.StackView.Init(v, false, storeName)
	v.SetMinSize(zgeo.SizeD(0, 22))
	v.SetSearchable(false)
	v.SetChildrenAboveParent(true)
	var items zdict.Items
	for _, a := range allTypes {
		if len(allowTypes) == 0 || zstr.StringsContain(allowTypes, a) {
			items = append(items, zdict.Item{Name: a, Value: a})
		}
	}
	v.actionMenu = zmenu.NewView("allow", items, storeKey)
	v.actionMenu.SetStoreKey(storeKey, Drop)
	v.actionMenu.SetSelectedHandler(func() {
		v.updateWidgets()
	})
	v.Add(v.actionMenu, zgeo.CenterLeft)

	textKey := storeKeyPrefix + "Text"
	text, _ := zkeyvalue.DefaultStore.GetString(textKey)
	tstyle := ztext.Style{KeyboardType: zkeyboard.TypeURL}
	v.mainText = ztext.NewView(text, tstyle, 30, 1)
	v.mainText.SetValueHandler("", func(edited bool) {
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
	v.password.SetValueHandler("", func(edited bool) {
		zkeyvalue.DefaultStore.SetString(v.password.Text(), passKey, true)
		v.updateWidgets()
	})
	v.password.UpdateSecs = 0
	v.Add(v.password, zgeo.CenterLeft)

	v.button = zbutton.New("")
	v.Add(v.button, zgeo.CenterLeft)
	v.button.SetPressedHandler("", zkeyboard.ModifierNone, v.buttonPressed)

	v.DropWell = zwidgets.NewDropWell("", zgeo.SizeD(10, 20))
	v.Add(v.DropWell, zgeo.CenterLeft|zgeo.Expand)
	v.DropWell.HandleDroppedFile = v.handleGivenFile
	v.DropWell.HandleDropPreflight = v.checkExtensions

	v.activity = zwidgets.NewActivityView(zgeo.SizeBoth(16), zgeo.ColorBlack)
	v.Add(v.activity, zgeo.CenterLeft)
}

func (v *UploadView) ReadyToShow(beforeWin bool) {
	if beforeWin {
		v.updateWidgets()
		if v.FileReadyToSendHandler == nil {
			v.FileReadyToSendHandler = v.CallHTTUpload
		}
	}
}

func (v *UploadView) checkExtensions(name string) bool {
	if len(v.AcceptExtensions) > 0 {
		ext := path.Ext(name)
		if !zstr.StringsContain(v.AcceptExtensions, ext) {
			zalert.ShowError(nil, "Incorrect file extension:", ext, v.AcceptExtensions)
			return false
		}
	}
	v.activity.Start()
	return true
}

func (v *UploadView) callFileReadyToSendHandler(up UploadPayload, data []byte) {
	v.activity.Start()
	v.FileReadyToSendHandler(up, data)
	v.activity.Stop()
}

func (v *UploadView) handleGivenFile(data []byte, name string) {
	var up UploadPayload
	up.HandleID = v.HandleID
	up.Name = name
	up.Type = v.actionMenu.CurrentValue().(string)
	go v.callFileReadyToSendHandler(up, data)
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
	go v.callFileReadyToSendHandler(up, nil)
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
	case Select:
		if v.selectButton == nil {
			v.addUploadButton()
		}
		ctext, cpass, cbutton, cdrop, cactivity = true, true, true, true, false
	}
	if action != Select && v.selectButton != nil {
		v.RemoveChild(v.selectButton, true)
		v.selectButton = nil
	}
	v.button.SetUsable(busable)
	v.button.SetText(tbutton)
	v.mainText.SetPlaceholder(ptext)
	v.CollapseChild(v.mainText, ctext, false)
	v.CollapseChild(v.password, cpass, false)
	v.CollapseChild(v.button, cbutton, false)
	v.CollapseChild(v.DropWell, cdrop, false)
	v.CollapseChild(v.activity, cactivity, false)
	v.ArrangeChildren()
}

func (v *UploadView) addUploadButton() {
	v.selectButton = zbutton.New("choose file")
	v.selectButton.SetMinWidth(100)
	len := v.CountChildren()
	v.AddAdvanced(v.selectButton, zgeo.CenterLeft, zgeo.RectNull, zgeo.SizeNull, len-1, false)
	v.selectButton.SetUploader(v.AcceptExtensions, v.handleGivenFile, v.checkExtensions, nil)
}

func (v *UploadView) CallHTTUpload(up UploadPayload, data []byte) {
	// TODO: Use zhttp_file.go routines
	var result zdict.Dict
	params := zhttp.MakeParameters()
	args := map[string]string{
		"id":   up.HandleID,
		"name": up.Name,
		"text": up.Text,
		"type": up.Type,
	}
	params.Headers["X-Password"] = up.Password
	if AuthTokenFunc != nil {
		params.Headers["X-Token"] = AuthTokenFunc()
	}
	// zlog.Info("CallHTTUpload:", AuthTokenFunc != nil, params.Headers["X-Token"])
	params.TimeoutSecs = float64(UploadTimeoutMinutes) * 60
	// params.PrintBody = true
	surl := zapp.DownloadPathPrefix + "zupload"
	surl, _ = zhttp.MakeURLWithArgs(surl, args)
	_, err := zhttp.Post(surl, params, data, &result)
	if err != nil {
		zalert.ShowError(err)
	}
	if result != nil {
		var err error
		serr, _ := result["error"].(string)
		if serr != "" {
			delete(result, "error")
			err = errors.New(serr)
		}
		if v.FileUploadedToServerHandler != nil {
			v.FileUploadedToServerHandler(result, err)
		} else if err != nil {
			zalert.ShowError(err)
		} else {
			message, got := result[ShowMessageKey]
			if got {
				zalert.Show(message)
			}
		}
		return
	}
	if v.FileUploadedToServerHandler != nil {
		v.FileUploadedToServerHandler(result, nil)
	}
}
