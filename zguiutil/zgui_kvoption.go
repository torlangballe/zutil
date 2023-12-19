//go:build zui

package zguiutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zui/zcheckbox"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zfloat"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zint"
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
	return v
}

func CreateViewForKVOption[V any](option *zkeyvalue.Option[V]) zview.View {
	var view zview.View
	v := option.Get()
	rval := reflect.ValueOf(v)
	kind := zreflect.KindFromReflectKindAndType(rval.Kind(), rval.Type())
	zlog.Info("CreateViewForKVOption:", option.Key, option.Get())
	switch kind {
	case zreflect.KindInt, zreflect.KindFloat:
		t := makeText(false, v, 10)
		t.SetChangedHandler(func() {
			str := t.Text()
			n, err := strconv.ParseFloat(str, 64)
			zlog.OnError(err, str)
			if err == nil {
				var v V
				rval := reflect.ValueOf(&v)
				if kind == zreflect.KindFloat {
					zfloat.SetAny(rval.Interface(), n)
				} else {
					zint.SetAny(rval.Interface(), int64(n))
				}
				option.Set(v)
			}
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
			var v V
			rval := reflect.ValueOf(&v)
			sptr := rval.Interface().(*string)
			// zlog.Info("SetString?:", rval.Type(), rval.Kind(), is)
			*sptr = text
			option.Set(v)
		})
		view = t

	case zreflect.KindBool:
		on := rval.Bool()
		check := zcheckbox.New(zbool.FromBool(on))
		check.SetValueHandler(func() {
			var v V
			rval := reflect.ValueOf(&v)
			bptr := rval.Interface().(*bool)
			*bptr = check.On()
			option.Set(v)
		})
		view = check
	}
	return view
}

func AddKVOptionToGrid[V any](grid *zcontainer.GridView, option *zkeyvalue.Option[V]) {
	name := zstr.PadCamelCase(option.Key, " ")
	label := zlabel.New(name)
	grid.Add(label, zgeo.CenterRight, zgeo.Size{})

	view := CreateViewForKVOption[V](option)
	grid.Add(view, zgeo.CenterLeft|zgeo.HorExpand, zgeo.Size{})
}
