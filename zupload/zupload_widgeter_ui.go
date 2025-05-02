//go:build zui

package zupload

import (
	"strings"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type UploadWidgeter struct{}

var (
	widgeterUploadHandlers   = map[string]func(up UploadPayload, data []byte){}
	widgeterUploadedHandlers = map[string]func(result zdict.Dict, err error){}
)

func RegisterWidget() {
	zfields.RegisterWidgeter("zupload", UploadWidgeter{})
}

func (a UploadWidgeter) Create(fv *zfields.FieldView, f *zfields.Field) zview.View {
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
	v := NewUploadView(f.ValueStoreKey, allow, f.ValueStoreKey)
	v.DropWell.SetPlaceholder(f.Placeholder)
	sext := f.CustomFields["ext"]
	if sext != "" {
		v.AcceptExtensions = strings.Split(sext, "|")
	}
	v.HandleID = f.CustomFields["handleid"]
	zlog.Assert(len(v.HandleID) > 0)
	zlog.Assert(v.HandleID != "")
	v.FileReadyToSendHandler = widgeterUploadHandlers[v.HandleID]
	v.FileUploadedToServerHandler = widgeterUploadedHandlers[v.HandleID]
	if f.Styling.FGColor.Valid {
		col := f.Styling.FGColor
		if col.Valid {
			v.SetColor(col)
		}
	}
	return v
}

func (a UploadWidgeter) SetupField(f *zfields.Field) {
	f.Flags |= zfields.FlagIsStatic
}

func SetWidgeterFileUploadHandler(id string, handler func(up UploadPayload, data []byte)) {
	widgeterUploadHandlers[id] = handler
}

func SetWidgeterFileUploadedHandler(id string, handler func(result zdict.Dict, err error)) {
	widgeterUploadedHandlers[id] = handler
}
