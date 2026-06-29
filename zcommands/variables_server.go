//go:build server

package zcommands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zobj"
	"github.com/torlangballe/zutil/zslices"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
)

type Variable struct {
	ID              int64       `zui:"-" zobj:"id"`
	UserID          int64       `zui:"-" zobj:"userid"`
	Name            string      `zui:"" zobj:"name"`
	ForTable        string      `zui:"-" zobj:"row"`
	OnlySubTypeName string      `zui:"-" zobj:"row"`
	ForInstance     int64       `zui:"-" zobj:"row"`
	ForProduct      string      `zui:"-" zui:"static" zobj:"row"`
	VarType         VarType     `zui:"-" zobj:"row"`
	Value           zdict.Items `zui:"" zobj:""`
}

type VarComs struct{}

type VarType string

type TableOwner interface {
	GetTable() string
}

type ProductIDOwner interface {
	GetProductID() string
}

const (
	VarNumber     VarType = "number"
	VarString     VarType = "string"
	VarNumberList VarType = "number-list"
	VarStringList VarType = "string-list"
	VarStringEnum VarType = "string-enum"
)

var VarTypes = []VarType{VarNumber, VarString, VarNumberList, VarStringList, VarStringEnum}

func InitVariablesServer() any {
	zobj.RegisterTableForSQL(&Variable{})
	return VarComs{}
}

// Command_addvar adds a variable to the current instance, subtype or product.
// It will be shown with ls/ll within it's current hierarchy, or for all of that subtype or product if not attached to a specific instance.
func (VarComs) Command_addvar(c *CommandInfo, a struct {
	Name        string `zui:""`
	Description string `zui:"desc:addvar <name> -- Adds a variable, prompting to attaching it to current instance, subtype or product."`
}) {
	var v Variable
	v.Name = a.Name

	instance := c.Session.CurrentNodeValue()
	stype, st, _ := zobj.LookupStructTableFromStruct(instance)
	if st != nil {
		v.ForTable = st.Table
		if st.IsSubType {
			v.OnlySubTypeName = stype.String()
		}
	} else {
		to, _ := instance.(TableOwner)
		if to != nil {
			v.ForTable = to.GetTable()
		}
	}
	if v.ForTable == "" {
		c.Session.TermSession.Write(zstr.EscYellow + "No table to add to in this type" + zstr.EscNoColor + " ")
		return
	}
	po, _ := instance.(ProductIDOwner)
	if po != nil {
		prodID := po.GetProductID()
		if prodID != "" {
			ask := zstr.Spaced("For product", prodID, "only? y/n")
			c.Session.TermSession.Write(zstr.EscYellow + ask + zstr.EscNoColor + " ")
			str, _ := c.Session.TermSession.ReadValueLine()
			if str == "" {
				c.Session.TermSession.Writeln("Empty input. Stopping.")
				return
			}
			if strings.ToLower(str) == "y" {
				v.ForProduct = prodID
			}
		}
	}
	id, _ := zobj.GetIDInStruct(instance, "id")
	if v.ForProduct == "" && id != 0 {
		c.Session.TermSession.Write(zstr.EscYellow + "For this instance only? y/n" + zstr.EscNoColor + " ")
		str, _ := c.Session.TermSession.ReadValueLine()
		if str == "" {
			c.Session.TermSession.Writeln("Empty input. Stopping.")
			return
		}
		if strings.ToLower(str) == "y" {
			v.ForInstance = id
		}
	}
	for i, v := range VarTypes {
		name := strings.Replace(string(v), "-", " ", -1)
		name = zstr.TitledWords(name)
		c.Session.TermSession.Writeln(i+1, name)
	}
	c.Session.TermSession.Write(zstr.EscYellow + "Choose Value Type from above" + zstr.EscNoColor + " ")
	snum, _ := c.Session.TermSession.ReadValueLine()
	if snum == "" {
		c.Session.TermSession.Writeln("Empty input. Stopping.")
		return
	}
	num, err := strconv.Atoi(snum)
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
	}
	num--
	if num < 0 || num >= len(VarTypes) {
		c.Session.TermSession.Writeln("Index outside range")
		return
	}
	v.VarType = VarTypes[num]
	if !keepAskingForNewVariableElement(c.Session, &v) {
		return
	}
	uid := c.Session.TermSession.UserID()
	_, err = zobj.Insert(&v, uid, uid)
	if err != nil {
		c.Session.TermSession.Writeln("Error saving variable:", err)
		return
	}
	c.Session.TermSession.Writeln("Variable '" + a.Name + "' added")
}

func getField(s *Session, v *Variable, verb string) (zdict.Item, bool) {
	stype := "Value"
	switch v.VarType {
	case VarNumber, VarNumberList:
		stype = "Number"
	case VarString, VarStringList:
		stype = "String"
	}
	str := fmt.Sprintf("%s %s (return to stop): ", verb, stype)
	s.TermSession.Write(zstr.EscYellow + str + zstr.EscNoColor)
	sval, _ := s.TermSession.ReadValueLine()
	if sval == "" {
		s.TermSession.Writeln(zstr.EscYellow + "Empty input. Stopping." + zstr.EscNoColor)
		return zdict.Item{}, false
	}
	var sname string
	if v.VarType == VarStringEnum {
		str := "Name: (return to stop): "
		s.TermSession.Write(zstr.EscYellow + str + zstr.EscNoColor)
		sname, _ = s.TermSession.ReadValueLine()
		if sname == "" {
			s.TermSession.Writeln(zstr.EscYellow + "Empty name. Stopping." + zstr.EscNoColor)
			return zdict.Item{}, false
		}
	}
	switch v.VarType {
	case VarNumber, VarNumberList:
		n, err := strconv.ParseFloat(sval, 64)
		if err != nil {
			s.TermSession.Writeln(err)
			return zdict.Item{}, false
		}
		return zdict.Item{Value: n}, true
	case VarString, VarStringList:
		return zdict.Item{Value: sval}, true
	case VarStringEnum:
		return zdict.Item{Name: sname, Value: sval}, true
	}
	return zdict.Item{}, false
}

func keepAskingForNewVariableElement(s *Session, v *Variable) bool {
	for {
		item, ok := getField(s, v, "Add")
		if !ok {
			return len(v.Value) > 0
		}
		v.Value = append(v.Value, item)
		if v.VarType == VarNumber || v.VarType == VarString {
			return true
		}
	}
}

func (v *Variable) String() string {
	var str string
	switch v.VarType {
	case VarNumber, VarString:
		if len(v.Value) == 0 {
			return ""
		}
		str = fmt.Sprint(v.Value[0].Value)
	case VarNumberList, VarStringList, VarStringEnum:
		for _, val := range v.Value {
			if str != "" {
				str += ", "
			}
			if v.VarType == VarStringEnum {
				str += val.Name + ":"
			}
			str += fmt.Sprint(val.Value)
		}
		str = "[" + str + "]"
	}
	return str
}

func (v *Variable) SetName(name string) {
	v.Name = name
}

func (v *Variable) CommandColumns() zdict.Items {
	const maxWidth = 80
	var items zdict.Items
	items.Add("type", v.VarType)
	str := zstr.TruncatedMiddle(v.String(), maxWidth, "…")
	items.Add("value", str)
	return items
}

func VariableNodesForStruct(s *Session, nodeInstance any, shelfOnly bool) []Node {
	var nodes []Node
	vars := GetVariablesForInstance(nodeInstance, s.TermSession.UserID(), "", shelfOnly)
	for _, v := range vars {
		set := v
		n := MakeNode(set.Name, VariableNode, &set, v.ID)
		nodes = append(nodes, n)
	}
	return nodes
}

func (vc VarComs) VariableNodesForStruct(s *Session, nodeInstance any, shelfOnly bool) []Node {
	return VariableNodesForStruct(s, nodeInstance, shelfOnly)
}

func GetVariableForInstance(nodeInstance any, userID int64, name string, shelfOnly bool) (Variable, bool) {
	vars := GetVariablesForInstance(nodeInstance, userID, name, shelfOnly)
	if len(vars) > 0 {
		return vars[0], true
	}
	return Variable{}, false
}

func GetVariablesForInstance(nodeInstance any, userID int64, name string, shelfOnly bool) []Variable {
	var w []string
	stype, st, _ := zobj.LookupStructTableFromStruct(nodeInstance)
	var table string
	str := "onlysubtypename=''"
	if st != nil {
		table = st.Table
		if st.IsSubType {
			str = fmt.Sprintf("(%s OR onlysubtypename='%s')", str, stype.String())
		}
	} else {
		to, _ := nodeInstance.(TableOwner)
		if to != nil {
			table = to.GetTable()
		}
	}
	w = append(w, str)
	if table != "" || shelfOnly {
		tw := fmt.Sprintf("fortable=''")
		if !shelfOnly {
			tw = fmt.Sprintf("fortable='%s'", table)
		}
		w = append(w, tw)
	}
	po, _ := nodeInstance.(ProductIDOwner)
	str = "forproduct=''"
	if po != nil {
		prodID := po.GetProductID()
		str = fmt.Sprintf("(%s OR forproduct='%s')", str, prodID)
	}
	w = append(w, str)
	id, _ := zobj.GetIDInStruct(nodeInstance, "id")
	str = "forinstance=0"
	if id != 0 {
		str = fmt.Sprintf("(%s OR forinstance=%d)", str, id)
	}
	w = append(w, str)
	w = append(w, fmt.Sprintf("userid=%d", userID))
	if name != "" {
		str := zsql.QuoteString(name)
		w = append(w, fmt.Sprintf("name=%s", str))
	}
	var vars []Variable
	where := strings.Join(w, " AND ")
	zlog.Info("GetVariablesForInstance:", where)
	err := zobj.Select(&vars, userID, userID, where)
	if zlog.OnError(err, where) {
		return nil
	}
	return vars
}

func (v *Variable) Edit(s *Session) {
	if v.VarType == VarNumber || v.VarType == VarString {
		if len(v.Value) > 0 {
			s.TermSession.Writeln("Current value:", v.Value[0].Value)
			s.TermSession.Writeln(zstr.EscYellow+"clear", zstr.EscNoColor)
		}
		s.TermSession.Writeln(zstr.EscYellow+"set", zstr.EscNoColor)
		s.TermSession.Write("command (return to stop): ")
		command, _ := s.TermSession.ReadValueLine()
		switch command {
		case "clear":
			v.Value = nil
			v.Update(s)
		case "set":
			item, ok := getField(s, v, "Set")
			if !ok {
				return
			}
			v.Value = []zdict.Item{item}
			v.Update(s)
		}
		return
	}
	var changed bool
loop:
	for {
		for i, v := range v.Value {
			str := fmt.Sprintf("%d) %s: %v", i+1, v.Name, v.Value)
			s.TermSession.Writeln(str)
		}
		if len(v.Value) > 0 {
			s.TermSession.Writeln(zstr.EscYellow+"edit", zstr.EscNoColor, "<index>")
			s.TermSession.Writeln(zstr.EscYellow+"rm", zstr.EscNoColor, "<index>")
		}
		s.TermSession.Writeln(zstr.EscYellow+"add", zstr.EscNoColor, "<Value>")
		s.TermSession.Write("command (return to stop): ")
		str, _ := s.TermSession.ReadValueLine()
		parts := strings.Fields(str)
		if len(parts) != 2 {
			s.TermSession.Writeln("Wrong number of arguments")
			return
		}
		command := parts[0]
		n, err := strconv.Atoi(parts[1])
		if command == "edit" || command == "rm" {
			if err != nil {
				s.TermSession.Writeln(err)
				continue
			}
			if n < 1 || n > len(v.Value) {
				s.TermSession.Writeln("Index out of range")
				continue
			}
		}
		switch command {
		case "edit":
			item, ok := getField(s, v, "Edit")
			if !ok {
				break loop
			}
			v.Value[n-1] = item
			changed = true
		case "rm":
			zslices.RemoveAt(&v.Value, n-1)
			changed = true
		case "add":
			var item zdict.Item
			if v.VarType == VarNumberList {
				item.Value, err = strconv.Atoi(parts[1])
				if err != nil {
					s.TermSession.Writeln(err)
					break loop
				}
			} else {
				item.Value = parts[1]
			}
			if v.VarType == VarStringEnum {
				s.TermSession.Write("Name (return to stop): ")
				item.Name, _ = s.TermSession.ReadValueLine()
				if item.Name == "" {
					s.TermSession.Writeln("Empty name. Stopping.")
					break loop
				}
			}
			v.Value = append(v.Value, item)
			changed = true
		default:
			s.TermSession.Writeln("Unknown command")
			break
		}
	}
	if changed {
		v.Update(s)
	}
}

func (v *Variable) Update(s *Session) {
	uid := s.TermSession.UserID()
	err := zobj.Update(v, uid, uid, "") // no where means set for id
	if err != nil {
		fmt.Fprintln(s.TermSession.Writer(), err)
	}
}

func (v *Variable) Delete(uid int64) error {
	err := zobj.Delete[Variable]([]int64{v.ID}, uid, "")
	if err != nil {
		return err
	}
	return nil
}

func DeleteVariablesForInstance(instance any, uid int64) {
	stype, st, err := zobj.LookupStructTableFromStruct(instance)
	if st == nil {
		zlog.Info("DeleteVariablesForInstance: No struct table for instance", stype)
		return
	}
	id, err := zobj.GetIDInStruct(instance, "id")
	if err != nil {
		zlog.Info("DeleteVariablesForInstance: No id field for instance", stype)
		return
	}
	query := fmt.Sprintf("DELETE FROM variables WHERE userid=$1 AND fortable=$2 AND forinstance=$3")
	_, err = zsql.Main.DB.Exec(query, uid, st.Table, id)
	if err != nil {
		zlog.Info("DeleteVariablesForInstance: Error deleting variables for instance", stype, err)
	}
}
