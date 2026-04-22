//go:build server

package zcommands

import (
	"fmt"
	"io"
	"reflect"
	"strconv"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
)

type editField struct {
	value reflect.Value
	field *zfields.Field
	key   string
}

const checkedString = zstr.EscMagenta + " [√]" + zstr.EscNoColor

func editFieldNode(c *CommandInfo, node Node) {
	parentNode := c.Session.TopNode()
	rval := node.editField.value

	kind := zreflect.KindFromReflectKindAndType(rval.Kind(), rval.Type())
	if kind == zreflect.KindPointer {
		node.editField.value = reflect.New(rval.Type().Elem()).Elem()
	}

	if kind == zreflect.KindSlice {
		editSliceIndicator(c, *parentNode, node)
		return
	}
	if node.editField.field.HasFlag(zfields.FlagEmptyEnum) {
		editEnumInterface(c, *parentNode, node)
		return
	}
	if node.editField.field.Enum != "" {
		editEnumIndicator(c, *parentNode, node)
		return
	}
	if node.editField.field.LocalEnum != "" {
		editLocalEnumIndicator(c, *parentNode, node)
		return
	}
	c.Session.TermSession.Write(zstr.EscGreen+node.editField.field.Name+zstr.EscNoColor, ": ")
	sval, err := c.Session.TermSession.ReadValueLine()
	if err == io.EOF {
		return
	}
	if sval == "" {
		c.Session.TermSession.Writeln("Empty input. Aborting edit. Add single space to set to empty.")
		return
	}
	if sval == " " {
		c.Session.TermSession.Writeln("Single space input. Setting to empty.")
		sval = ""
	}

	switch kind {
	case zreflect.KindString:
		zlog.Info("editNode set string:", sval, node.editField.value.Type(), node.editField.value.CanAddr(), parentNode.Name, zlog.Full(parentNode.Instance))
		node.editField.value.SetString(sval)
		//! c.Session.callUpdate(s)
	case zreflect.KindInt, zreflect.KindFloat:
		n, err := strconv.ParseFloat(sval, 64)
		if err != nil {
			c.Session.TermSession.Writeln(err)
			return
		}
		if kind == zreflect.KindInt {
			node.editField.value.SetInt(int64(n))
		} else {
			node.editField.value.SetFloat(n)
		}
		//! c.Session.callUpdate(s)
	}
	zlog.Info("Edited:", parentNode.Name)
	callUpdater(c, *parentNode)
}

func editEnumInterface(c *CommandInfo, parentNode, node Node) {
	eg, _ := parentNode.Instance.(zfields.EnumGetter)
	if zlog.ErrorIf(eg == nil, "editEnumInterface: field is empty enum but does not implement EnumGetter", node.editField.field.Name, reflect.TypeOf(node.Instance)) {
		return
	}
	enum := eg.GetEnum(node.editField.field.FieldName)
	editAnyEnumIndicator(c, parentNode, node, enum)
}

func editEnumIndicator(c *CommandInfo, parentNode, node Node) {
	enum := zfields.GetEnum(node.editField.field.Enum)
	editAnyEnumIndicator(c, parentNode, node, enum)
}

func callUpdater(c *CommandInfo, parentNode Node) {
	updater, _ := parentNode.Instance.(Updater)
	zlog.Info("callUpdater for parent node:", parentNode.Name, "type:", reflect.TypeOf(parentNode.Instance), "updater:", updater != nil)
	if updater != nil {
		updater.Update(c.Session)
	}
}

func editLocalEnumIndicator(c *CommandInfo, parentNode, node Node) {
	// zlog.Info("editLocalEnumIndicator:", node.Instance, zlog.Full(*node.editField.field))
	ei, findex := zfields.FindLocalFieldWithFieldName(parentNode.Instance, node.editField.field.LocalEnum)
	zlog.Assert(findex != -1, node.editField.field.Name, node.editField.field.LocalEnum)
	enum := ei.Interface().(zdict.ItemsGetter).GetItems()
	editAnyEnumIndicator(c, parentNode, node, enum)
}

func editAnyEnumIndicator(c *CommandInfo, parentNode, node Node, enum zdict.Items) {
	for i, e := range enum {
		c.Session.TermSession.Write(i+1, ") ", e.Name)
		if node.editField.value.Equal(reflect.ValueOf(e.Value)) {
			c.Session.TermSession.Write(" " + checkedString)
		}
		c.Session.TermSession.Writeln("")
	}
	doRepeatEditIndex(c, "setting index", "Set Index No:", len(enum), func(n int) bool {
		node.editField.value.Set(reflect.ValueOf(enum[n].Value))
		c.Session.TermSession.Writeln(zstr.EscGreen+node.editField.field.Name, zstr.EscNoColor+"set to", enum[n].Name)
		//! c.Session.callUpdate(s)
		return true
	})
	callUpdater(c, parentNode)
}

func editSliceIndicator(c *CommandInfo, parentNode, node Node) bool {
	sliceVal := node.editField.value
	if sliceVal.Kind() == reflect.Pointer {
		sliceVal = sliceVal.Elem()
	}
	length := sliceVal.Len()
	zlog.Assert(length != 0)
	indicatorName := zfields.FindIndicatorOfSlice(sliceVal.Interface())
	c.Session.TermSession.Writeln("Set", zstr.EscCyan+node.editField.field.Name+zstr.EscNoColor, "index:")
	lastUsedID, _ := zkeyvalue.DefaultStore.GetString(node.editField.key)
	var ids, titles []string
	for j := 0; j < length; j++ {
		a := sliceVal.Index(j).Addr().Interface()
		id := zstr.GetIDFromAnySliceItemWithIndex(a, j)
		finfo, found := zreflect.FieldForName(a, zfields.FlattenIfAnonymousOrZUITag, indicatorName)
		var title string
		if !found {
			title = fmt.Sprint(j + 1)
		} else {
			title = fmt.Sprint(finfo.ReflectValue.Interface())
		}
		ids = append(ids, id)
		titles = append(titles, title)
		var current string
		if id == lastUsedID || lastUsedID == "" && j == 0 {
			current = checkedString
		}
		c.Session.TermSession.Write(j+1, ") ", title, current, "\n")
	}
	doRepeatEditIndex(c, "setting index", "Set Index No:", length, func(n int) bool {
		// zlog.Info("SetIndexForSlice:", ids[n], n)
		zkeyvalue.DefaultStore.SetString(ids[n], node.editField.key, true)
		c.Session.TermSession.Writeln("Set index for", zstr.EscCyan+node.editField.field.Name+zstr.EscNoColor, "to:", titles[n])
		// c.Session.callUpdate(s)
		return true
	})
	callUpdater(c, parentNode)
	return true
}

func doRepeatEditIndex(c *CommandInfo, what, prompt string, length int, do func(n int) (quit bool)) {
	c.Session.TermSession.Writeln("Press ["+zstr.EscYellow+"return"+zstr.EscNoColor+"] to quit", what)
	for {
		c.Session.TermSession.Write(zstr.EscYellow + prompt + zstr.EscNoColor + " ")
		sval, _ := c.Session.TermSession.ReadValueLine()
		if sval == "" {
			break
		}
		n, _ := strconv.Atoi(sval)
		if n <= 0 || n > length { // n is 1 -- x
			c.Session.TermSession.Writeln("Field number outside range")
			continue
		}
		if do(n - 1) {
			break
		}
	}
}

func (defaultCommands) Expand_edit(c *CommandInfo, line string, add *string) {
	ExpandPath(c.Session, line, add, FieldNode)
	// zlog.Info("Expand_cd2", line, *add)
}

func (defaultCommands) Command_edit(c *CommandInfo, a struct {
	Path        string
	Description string `zui:"desc:Edit a field/variable. Use path to specify name or number of listed items."`
}) {
	node, err := c.Session.PathAsItsTopNode(a.Path, FieldNode)
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
	}
	if node.Type == VariableNode {
		eo, _ := node.Instance.(Editer)
		if eo != nil {
			eo.Edit(c.Session)
			callUpdater(c, node)
			return
		}
	}
	if node.Type != FieldNode {
		c.Session.TermSession.Writeln("Node is not a field/variable:", a.Path)
		return
	}
	editFieldNode(c, node)
}

func (defaultCommands) Command_setname(c *CommandInfo, a struct {
	Path        string
	Name        string
	Description string `zui:"desc:setname <path> <name> --Set name of a variable. Use path to specify name or number of listed items."`
}) {
	node, err := c.Session.PathAsItsTopNode(a.Path, FieldNode)
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
	}
	if node.Type != VariableNode {
		c.Session.TermSession.Writeln("Node is not a field/variable:", a.Path)
		return
	}
	if a.Name == "" {
		c.Session.TermSession.Writeln("No name given")
		return
	}
	ns, is := node.Instance.(zstr.NameSetter)
	if zlog.ErrorIf(!is, reflect.TypeOf(node.Instance)) {
		return
	}
	ns.SetName(a.Name) // it's a variables.Variable, but we can't include that here
	callUpdater(c, node)
	c.Session.TermSession.Writeln(zstr.EscYellow+node.Name, `name changed to: "`+a.Name+`"`, zstr.EscNoColor)
}
