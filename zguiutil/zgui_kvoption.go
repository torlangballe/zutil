//go:build zui

package zguiutil

import (
	"fmt"
	"math/rand"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

func makeText(isPassword bool, val any, cols int) *ztext.TextView {
	str := fmt.Sprint(val)
	style := ztext.Style{}
	if isPassword {
		style.KeyboardType = zkeyboard.TypePassword
	}
	v := ztext.NewView(str, style, cols, 1)
	v.UpdateSecs = 4
	return v
}

func CreateViewForKVOption[V comparable](option *zkeyvalue.Option[V]) zview.View {
	var view zview.View
	v := option.Get()
	rval := reflect.ValueOf(v)
	kind := zreflect.KindFromReflectKindAndType(rval.Kind(), rval.Type())
	id := rand.Int63()
	// zlog.Info("CreateViewForKVOption:", option.Key, option.Get(), zlog.Pointer(option))
	switch kind {
	case zreflect.KindInt, zreflect.KindFloat:
		t := makeText(false, v, 10)
		t.SetChangedHandler(func() {
			str := t.Text()
			n, err := strconv.ParseFloat(str, 64)
			zlog.OnError(err, str)
			if err == nil {
				option.SetAny(n, true)
			}
		})
		zkeyvalue.AddOptionChangedHandler(id, option.Key, func() {
			zlog.Info("CreateViewForKVOption changed number:", option.Key, option.Get())
			t.SetText(fmt.Sprint(option.Get()))
		})
		view = t

	case zreflect.KindString:
		var isPass bool
		for _, n := range []string{"Secret", "APIKey", "Password"} {
			if strings.Contains(option.Key, n) {
				isPass = true
			}
		}
		// zlog.Info("CreateView:", isPass, option.Key)
		t := makeText(isPass, v, 40)
		t.SetChangedHandler(func() {
			text := t.Text()
			option.SetAny(text, true)
		})
		zkeyvalue.AddOptionChangedHandler(id, option.Key, func() {
			zlog.Info("CreateViewForKVOption changed string:", option.Key, option.Get())
			t.SetText(fmt.Sprint(option.Get()))
		})
		view = t

	case zreflect.KindBool:
		on := rval.Bool()
		check := zcheckbox.New(zbool.FromBool(on))
		check.SetValueHandler(func() {
			option.SetAny(on, true)
		})
		zkeyvalue.AddOptionChangedHandler(id, option.Key, func() {
			zlog.Info("CreateViewForKVOption changed bool:", option.Key, option.Get())
			bval := reflect.ValueOf(option.Get())
			check.SetOn(bval.Bool())
		})
		view = check
	}
	return view
}

func AddKVOptionToGrid[V comparable](grid *zcontainer.GridView, option *zkeyvalue.Option[V]) {
	name := zstr.PadCamelCase(option.Key, " ")
	view := CreateViewForKVOption[V](option)
	AddLabeledViewToGrid(grid, name, view)
}
