//go:build zui

package zupload

import (
	"strings"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type UploadWidgeter struct{}

var (
	widgeterHandlers = map[string]func(up UploadPayload, data []byte){}
)

func RegisterWidget() {
	zfields.RegisterWidgeter("zupload", UploadWidgeter{})
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
	v := NewUploadView(f.ValueStoreKey, allow)
	v.DropWell.SetPlaceholder(f.Placeholder)
	sext := f.CustomFields["ext"]
	if sext != "" {
		v.AcceptExtensions = strings.Split(sext, "|")
	}
	v.HandleID = f.CustomFields["handleid"]
	zlog.Assert(len(v.HandleID) > 0)
	zlog.Assert(v.HandleID != "")
	v.FileReadyToSendHandler = widgeterHandlers[v.HandleID]
	zlog.Info("Create:", v.CallHTTUpload != nil)
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

func SetWidgeterFileHandler(id string, handler func(up UploadPayload, data []byte)) {
	widgeterHandlers[id] = handler
}
