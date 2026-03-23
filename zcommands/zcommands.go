//go:build server

package zcommands

import (
	"fmt"
	"os/user"
	"reflect"
	"strings"
	"time"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/zwords"
)

type ArgType string
type FileArg string
type CommandType string

// type Arg struct {
// 	Name       string
// 	Type       ArgType
// 	IsOptional bool
// 	IsFlag     bool
// }

type Commander struct {
	sessions map[string]*Session
	// rootNode    any
	GlobalComs []any
}

type CommandInfo struct {
	Session *Session
	Type    CommandType
}

type Initer interface {
	Init()
}

type CanRunner interface {
	CanRunCommands() bool
}

type ColumnOwner interface {
	CommandColumns() zdict.Items
}

type VariableCreator interface {
	VariableNodesForStruct(s *Session, nodeInstance any) []Node
}

type NodeType int

const (
	FieldNode NodeType = 1 << iota
	StaticFieldNode
	RowNode
	ComNode
	MethodNode
	EnumNode
	VariableNode
)

type Node struct {
	Name        string
	Type        NodeType
	Description string
	Instance    any
	FieldSVal   string
	editField   editField
	id          int64
	parentID    int64 // parentID is used to show a hierarchy
}

type Updater interface {
	Update(s *Session)
}

type Deleter interface {
	Delete(uid int64) error
}

func MakeNode(name string, ntype NodeType, instance any, id int64) Node {
	n := Node{Name: name, Type: ntype, Instance: instance, id: id}
	return n
}

type NodeOwner interface {
	CommandNodes(s *Session, where string, forExpand bool) []Node
}

type Editer interface {
	Edit(s *Session)
}

type SliceNodeMaker[S any] struct {
	slicePtr *[]S
}

func (m SliceNodeMaker[S]) CommandNodes(s *Session, where string, forExpand bool) []Node {
	var nodes []Node
	slice := *m.slicePtr
	for i := range slice {
		item := slice[i]
		nodes = append(nodes, MakeNode(fmt.Sprint(i), RowNode, &item, 0))
	}
	return nodes
}

var (
	AllowBash       bool
	AddressIP4      string
	commandInfoType = reflect.TypeOf(&CommandInfo{})
)

const lastCDKVKey = "zcommand.CD.Path"

func NewCommander(rootNode any, term *zterm.Terminal) *Commander {
	c := new(Commander)
	// c.rootNode = rootNode
	c.sessions = map[string]*Session{}
	c.GlobalComs = []any{defaultCommands{}}
	term.HandleNewSession = func(ts *zterm.Session) func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		s := new(Session)
		s.commander = c
		s.id = ts.ContextSessionID()
		nn := MakeNode("/", ComNode, rootNode, 0)
		s.NodeHistory = []Node{nn}
		s.TermSession = ts
		c.sessions[s.id] = s
		if zkeyvalue.DefaultStore != nil {
			path, _ := zkeyvalue.DefaultStore.GetString(lastCDKVKey)
			if path != "" {
				s.changeDirectory(path)
			}
		}
		s.updatePrompt()
		return s.autoComplete
	}
	return c
}

func (c *Commander) HandleTerminalLine(line string, ts *zterm.Session) bool {
	if line == "close" {
		return false
	}
	c.HandleLine(line, ts)
	return true
}

func (c *Commander) HandleLine(line string, ts *zterm.Session) {
	sessionID := ts.ContextSessionID()
	s := c.sessions[sessionID]
	s.doCommand(line, false)
}

func findMethodByName(structure any, methodName string) (reflect.Method, bool) {
	for _, st := range anonStructsAndSelf(structure) {
		rval := reflect.ValueOf(st)
		t := rval.Type()
		for m := 0; m < t.NumMethod(); m++ {
			method := t.Method(m)
			if method.Name == methodName {
				return method, true
			}
		}
	}
	return reflect.Method{}, false
}

func anonStructsAndSelf(structure any) []any {
	anon := []any{structure}
	zreflect.ForEachField(structure, nil, func(each zreflect.FieldInfo) bool {
		if each.StructField.Anonymous {
			if each.ReflectValue.CanAddr() {
				each.ReflectValue = each.ReflectValue.Addr()
			}
			anon = append(anon, each.ReflectValue.Interface())
		}
		return true
	})
	return anon
}

type Help struct {
	Method      string
	Description string
	Args        []zstr.KeyValue
}

func CreateCommanderAndTerminal(welcome string, keysdir string, hardUsers map[string]string, rootNode any, port int, requireSystemPasswordIfNoZUser bool) *Commander {
	_, ip, _ := znet.GetCurrentLocalIPAddress()
	if ip == "" {
		ip = "localhost"
	}
	user, _ := user.Current()
	if port == 0 {
		port = 2222
	}
	zlog.Info("Command line interface connected.", "ssh", user.Username+"@"+ip, "-p", port)
	terminal := zterm.New(welcome + ". Type 'close' or press control-D to exit.")
	if keysdir != "" {
		terminal.PublicKeyStorePath = keysdir + "terminal-pubkeys.json"
	}
	terminal.RequireSystemPasswordIfNoZUser = requireSystemPasswordIfNoZUser
	for user, pass := range hardUsers {
		terminal.AddHardcodedUser(user, pass)
	}
	commander := NewCommander(rootNode, terminal)
	terminal.HandleLine = commander.HandleTerminalLine
	go terminal.ListenForever(port)
	return commander
}

func NodesForStruct(s *Session, instance any, part string, nodeTypes NodeType, longList bool) []Node {
	var nodes []Node
	if zreflect.NonPointerKind(instance) != reflect.Struct {
		// fmt.Fprintln(s.TermSession.Writer(), "command on non-struct:", reflect.TypeOf(instance))
		return nil
	}
	if nodeTypes&ComNode != 0 {
		zreflect.ForEachField(instance, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
			if each.ReflectValue.Kind() == reflect.Pointer {
				each.ReflectValue = each.ReflectValue.Elem()
			}
			// getter, _ := each.ReflectValue.Interface().(NodeOwner)
			// zlog.Info("NodesForStruct", reflect.TypeOf(instance), each.ReflectValue.Type(), getter != nil, part, each.StructField.Name)
			// if getter != nil {
			// 	otherNodes := getter.CommandNodes(s, part, false)
			// 	nodes = append(nodes, otherNodes...)
			// }
			if each.ReflectValue.Kind() != reflect.Struct {
				return true
			}
			if !each.ReflectValue.CanAddr() {
				return true
			}
			commander := each.ReflectValue.Addr().Interface()
			meths := s.specialMethodNames(commander)
			if len(meths) == 0 {
				_, no := commander.(NodeOwner)
				_, io := commander.(Initer)
				_, do := commander.(zstr.Describer)
				if !no && !io && !do {
					return true // go to next
				}
			}
			name := strings.ToLower(each.StructField.Name)
			// zlog.Info("NodesForStruct", reflect.TypeOf(instance), part, name)
			// initCommander(commander)
			nodes = append(nodes, MakeNode(name, ComNode, commander, 0))
			return true
		})
		getter, _ := instance.(NodeOwner)
		if getter != nil {
			otherNodes := getter.CommandNodes(s, part, false)
			nodes = append(nodes, otherNodes...)
		}
	}
	if nodeTypes&FieldNode != 0 {
		fnodes := fieldNodes(s, instance, part, 0, false)
		nodes = append(nodes, fnodes...)
	}
	if nodeTypes&MethodNode != 0 {
		mnodes := methodNodes(instance)
		nodes = append(nodes, mnodes...)
	}
	if nodeTypes&VariableNode != 0 {
		for _, gc := range s.commander.GlobalComs {
			vc, _ := gc.(VariableCreator)
			if vc != nil {
				vnodes := vc.VariableNodesForStruct(s, instance)
				nodes = append(nodes, vnodes...)
			}
		}
	}
	return nodes
}

func fieldNodes(s *Session, instancePtr any, part string, indent int, inStatic bool) []Node {
	var nodes []Node
	var path string
	params := zfields.FieldParameters{}
	// zlog.Info("fieldNodes:", reflect.TypeOf(instancePtr), zlog.Full(instancePtr))
	zfields.ForEachField(instancePtr, params, nil, func(each zfields.FieldInfo) bool {
		// col := indentColors[indent%len(indentColors)]
		if each.Field.Flags&zfields.FlagIsButton != 0 || each.Field.Flags&zfields.FlagIsImage != 0 {
			if !each.Field.IsImageToggle() {
				return true // skip
			}
		}
		kind := zreflect.KindFromReflectKindAndType(each.ReflectValue.Kind(), each.ReflectValue.Type())
		sval, skip := getValueString(instancePtr, each.ReflectValue, each.Field, each.StructField, 3000, false)
		// zlog.Info("fieldNodes2:", each.StructField.Name, each.ReflectValue.Interface(), skip, sval)
		if skip {
			return true
		}
		var readOnlyStruct bool
		if each.ReflectValue.Kind() == reflect.Struct {
			if sval == "" {
				fnodes := fieldNodes(s, each.ReflectValue.Addr().Interface(), path+"/"+each.StructField.Name, indent+1, each.Field.IsStatic())
				nodes = append(nodes, fnodes...)
				return true
			}
			if each.Field.LocalEnum == "" {
				_, got := each.ReflectValue.Interface().(zfields.UISetStringer)
				readOnlyStruct = !got
			}
		}

		if kind == zreflect.KindSlice && each.ReflectValue.Type().Elem().Kind() == reflect.Struct {
			if each.ReflectValue.Type() != reflect.TypeOf(zdict.Items{}) {
				return true
			}
		}
		node := MakeNode(each.Field.TitleOrName(), FieldNode, each.ReflectValue.Interface(), 0)
		node.editField = editField{value: each.ReflectValue, field: each.Field}
		if each.Field.IsStatic() || inStatic || readOnlyStruct {
			node.Type = StaticFieldNode
		}
		node.FieldSVal = sval
		nodes = append(nodes, node)
		return true
	})
	return nodes
}

func getValueString(parent any, val reflect.Value, f *zfields.Field, sf reflect.StructField, maxChars int, setStructString bool) (str string, skip bool) {
	kind := zreflect.KindFromReflectKindAndType(val.Kind(), val.Type())
	zuis, _ := val.Interface().(zfields.UIStringer)
	// zlog.Info("GetValueString", f.Name, zuis != nil)
	if zuis != nil {
		return zuis.ZUIString(f.HasFlag(zfields.FlagAllowEmptyAsZero)), false
	}
	var sss struct{}
	if kind == zreflect.KindStruct {
		if sf.Type == reflect.TypeOf(sss) {
			return "", true
		}
		if !setStructString {
			return
		}
	}
	var enum zdict.Items
	istr := fmt.Sprint(val.Interface())
	if f.HasFlag(zfields.FlagEmptyEnum) {
		eg, _ := parent.(zfields.EnumGetter)
		if zlog.ErrorIf(eg == nil, "field is empty enum but does not implement EnumGetter", f.Name, val.Type()) {
			return "", false
		}
		enum = eg.GetEnum(f.FieldName)
		// zlog.Info("getValueString: got enum for field", f.Name, "enum:", zlog.Full(enum), "value:", istr)
	}
	if f.Enum != "" {
		enum = zfields.GetEnum(f.Enum)
	}
	if len(enum) > 0 {
		for _, e := range enum {
			if val.Equal(reflect.ValueOf(e.Value)) {
				return e.Name, false
			}
		}
		return "", false
	}

	if kind == zreflect.KindTime {
		t := val.Interface().(time.Time)
		str = ztime.GetNice(t, true)
	} else if kind == zreflect.KindSlice {
		str = sliceValueString(f, val, maxChars)
	} else {
		str = istr
	}
	return
}

func sliceValueString(f *zfields.Field, val reflect.Value, maxChars int) string {
	var str string
	slice, is := val.Interface().([]float64)
	if is && len(slice) <= 6 {
		var parts []string
		for _, f := range slice {
			parts = append(parts, zwords.NiceFloat(f, 1))
		}
		str = strings.Join(parts, " ")
	} else if val.Len() <= 4 && f.StringSep != "" {
		str = f.JoinSeparatedSlice(val)
	}
	if str != "" && (maxChars == 0 || len(str) <= maxChars) {
		return str
	}
	return zwords.Pluralize("item", val.Len())
}

func methodNodes(instance any) []Node {
	cr, canRun := instance.(CanRunner)
	if canRun && !cr.CanRunCommands() {
		return nil
	}
	var nodes []Node
	rval := reflect.ValueOf(instance)
	t := rval.Type()
	for m := 0; m < t.NumMethod(); m++ {
		var node Node
		node.Type = MethodNode
		method := t.Method(m)
		if zstr.LastByteAsString(method.Name) == "_" {
			continue
		}
		mtype := method.Type
		i := 1
		hasCommand := (mtype.NumIn() > 1 && mtype.In(1) == commandInfoType)
		if !hasCommand { // Other public funcs
			continue
		}
		if !zstr.HasPrefix(method.Name, "Command_", &node.Name) {
			continue
		}
		i++
		if i < mtype.NumIn() {
			atype := mtype.In(i)
			z := reflect.Zero(atype)
			// zlog.Info("Help:", method.Name, i, mtype.NumIn(), hasCommand)
			node.Description = zfields.GetDescriptionFromZUIField(z.Interface())
		}
		nodes = append(nodes, node)
	}
	return nodes
}

func ConcatWhereWithWild(where, wild string) string {
	if wild == "" {
		return where
	}
	w := strings.Replace(wild, "*", "%", -1)
	if w == wild {
		wild = "name=" + zsql.QuoteString(w)
	} else {
		wild = "ILIKE " + zsql.QuoteString(w)
	}
	if where == "" {
		return wild
	}
	return where + " AND (" + wild + ")"
}

func (n Node) Identifier() string {
	if n.Instance != nil {
		ider, ok := n.Instance.(zmath.Identifier)
		if ok {
			return ider.Identifier()
		}
	}
	return ""
}

func (n Node) IdentifierOfParent() string {
	if n.Instance != nil {
		ider, ok := n.Instance.(zmath.ParentChild)
		if ok {
			return ider.IdentifierOfParent()
		}
	}
	return ""
}
