//go:build server

package zcommands

import (
	"errors"
	"os/user"
	"reflect"
	"strings"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
)

type ArgType string
type FileArg string
type CommandType string

const (
	ArgOther  ArgType = ""
	ArgString ArgType = "string"
	ArgInt    ArgType = "int"
	ArgFloat  ArgType = "float"
	ArgBool   ArgType = "bool"

	CommandExecute CommandType = "execute"
	CommandHelp    CommandType = "help"
	CommandExpand  CommandType = "expand"

	TopHelp = `This command-line interface is a tree of nodes based on important
parts of an app; tables, main structures or functionality.
Use the "cd" command to move into a node, where "help" will show commands
specific to that node. Type ls to show child nodes.`
)

type Arg struct {
	Name       string
	Type       ArgType
	IsOptional bool
	IsFlag     bool
}

type namedNode struct {
	name string
	node any
}

type Session struct {
	id          string
	TermSession *zterm.Session
	promptExtra string
	nodeHistory []namedNode
	commander   *Commander
}

func (s *Session) SetPromptExtra(extra string) {
	s.promptExtra = extra
	s.updatePrompt()
}

type Commander struct {
	sessions map[string]*Session
	// rootNode    any
	GlobalNodes []any
}

type CommandInfo struct {
	Session *Session
	Type    CommandType
}

type methodNode struct {
	Name string
}

type Initer interface {
	Init()
}

type ColumnGetter interface {
	GetColumns() []zstr.KeyValue
}

type NodeOwner interface {
	GetChildrenNodes(s *Session, where, mode string, forExpand bool) map[string]any
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
	c.GlobalNodes = []any{&defaultCommands{}}
	term.HandleNewSession = func(ts *zterm.Session) func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		s := new(Session)
		s.commander = c
		s.id = ts.ContextSessionID()
		nn := namedNode{"/", rootNode}
		s.nodeHistory = []namedNode{nn}
		s.TermSession = ts
		c.sessions[s.id] = s
		// zlog.Info("LOADPATH?:", zkeyvalue.DefaultStore != nil)
		if zkeyvalue.DefaultStore != nil {
			path, _ := zkeyvalue.DefaultStore.GetString(lastCDKVKey)
			// zlog.Info("LOADPATH:", path)
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
	str := s.doCommand(line, CommandExecute)
	if str != "" {
		s.TermSession.Writeln(str)
	}
}

func (s *Session) TopNode() *namedNode {
	zlog.Assert(len(s.nodeHistory) != 0)
	return &s.nodeHistory[len(s.nodeHistory)-1]
}

func (s *Session) doCommand(line string, ctype CommandType) string {
	parts := zstr.GetQuotedArgs(line)
	if len(parts) == 0 {
		return ""
	}
	command := parts[0]
	args := parts[1:]
	// zlog.Info("doCommand", command, args)
	str, got := s.structCommand(s.currentNodeValue(), command, args, ctype)
	if got {
		return str
	}
	for i := len(s.commander.GlobalNodes) - 1; i >= 0; i-- { // we do it backward, so app-specific appended global node handles before default, so we can override
		n := s.commander.GlobalNodes[i]
		str, got = s.structCommand(n, command, args, ctype)
		if got {
			return str
		}
	}
	s.TermSession.Writeln("command not found:", command)
	return ""
}

func (s *Session) autoComplete(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	line = strings.TrimLeft(line, " ")
	if key == 9 {
		if strings.Contains(line, " ") {
			str := s.doCommand(line, CommandExpand)
			// if str == "" {
			// 	return
			// }
			ret := line + str
			return ret, len(ret), true
		}
		var names []string
		nodes := append(s.commander.GlobalNodes, s.currentNodeValue())
		for _, n := range nodes {
			names = append(names, s.specialMethodNames(n)...)
		}
		return s.expandForList(line, names, line)
	}
	return
}

func (s *Session) ExpandChildren(fileStub, prefix string) (addStub string) {
	n, _, _ := s.expandChildren(fileStub, prefix)
	return n
}

func (s *Session) expandChildren(fileStub, prefix string) (newLine string, newPos int, ok bool) {
	var names []string
	forExpand := true

	var dirs string
	stub := zstr.TailUntilWithRest(fileStub, "/", &dirs)
	// if dirs == "" {
	// 	dirs = strings.TrimLeft(fileStub, "/")
	// 	stub = ""
	// }
	stub = strings.TrimRight(stub, "/")

	var top any
	zlog.Info("expandChildren", dirs, "stub:", stub)
	if dirs == "" {
		top = s.currentNodeValue()
		if zstr.HasPrefix(fileStub, "/", &stub) {
			top = s.nodeHistory[0].node
		}
	} else {
		root := s.nodeHistory
		if zstr.HasPrefix(dirs, "/", &dirs) {
			root = s.nodeHistory[:1]
		}
		nodes, _ := s.findNodeInPath(root, dirs)
		zlog.Info("expandChildren2", dirs, nodes)
		top = s.currentNodeValue()
		if len(nodes) > 0 {
			if len(nodes) != 0 {
				top = nodes[len(nodes)-1].node
			}
		}
	}
	dirs = strings.TrimLeft(dirs, "/")
	var pre string
	for n := range s.getChildNodesOf(top, "", "", forExpand) {
		names = append(names, n) // zstr.Concat("/", dirs,
	}
	zlog.Info("expandChildren3", top, pre, names, stub, prefix, "dirs:", dirs, fileStub)
	return s.expandForList(stub, names, prefix)
}

func (s *Session) expandForList(stub string, list []string, prefix string) (newLine string, newPos int, ok bool) {
	zlog.Info("expandForList1", stub, list)
	var commands []string
	for _, c := range list {
		if strings.HasPrefix(c, stub) {
			commands = append(commands, c)
		}
	}
	if len(commands) == 0 {
		return
	}
	if len(commands) == 1 {
		var c string
		zstr.HasPrefix(commands[0], stub, &c)
		m := prefix + c
		return m, len(m), true
	}
	s.TermSession.Writeln("\n" + strings.Join(commands, " "))
	var add string
	diff := zstr.CommonExtremityOfSlice(commands, true)
	zstr.HasPrefix(diff, stub, &add)
	if diff != "" {
		zlog.Info("expandForList:", diff, add, stub, prefix, zdebug.CallingStackString())
		add = prefix + add
		return add, len(add), true
	}
	return
}

func (s *Session) currentNodeValue() any {
	return s.currentNode().node
}

func (s *Session) currentNode() namedNode {
	return s.nodeHistory[len(s.nodeHistory)-1]
}

func (s *Session) Path() string {
	var str string
	for _, n := range s.nodeHistory {
		if str != "" && str != "/" {
			str += "/"
		}
		str += n.name
	}
	return str
}

func (s *Session) updatePrompt() {
	path := s.Path()
	str := zstr.Concat(" ", path, s.promptExtra)
	s.TermSession.SetPrompt(str + "> ")
	if zkeyvalue.DefaultStore != nil {
		zkeyvalue.DefaultStore.SetString(path, lastCDKVKey, true)
	}
}

func (s *Session) structCommand(structure any, command string, args []string, ctype CommandType) (result string, got bool) {
	for _, st := range anonStructsAndSelf(structure) {
		rval := reflect.ValueOf(st)
		t := rval.Type()
		for m := 0; m < t.NumMethod(); m++ {
			method := t.Method(m)
			mtype := method.Type
			if mtype.NumIn() == 1 || mtype.In(1) != commandInfoType {
				continue
			}
			if strings.ToLower(method.Name) == command {
				return s.structCommandWithMethod(method, rval, args, ctype), true
			}
		}
	}
	return "", false
}

func (s *Session) structCommandWithMethod(method reflect.Method, structVal reflect.Value, args []string, ctype CommandType) string {
	mtype := method.Type
	sparams := []reflect.Value{structVal}
	i := 1
	hasCommand := (mtype.NumIn() > 1 && mtype.In(i) == commandInfoType && mtype.In(i).Kind() == reflect.Pointer)
	if hasCommand {
		c := &CommandInfo{Session: s, Type: ctype}
		sparams = append(sparams, reflect.ValueOf(c))
		i++
	}
	if i < mtype.NumIn() {
		argStructType := mtype.In(i)
		argVal := reflect.New(argStructType)
		// argValPtr := argVal
		// kind := argStructType.Kind()
		// if kind == reflect.Pointer {
		// 	kind = argStructType.Elem().Kind()
		// 	argVal = reflect.New(argStructType.Elem()).Elem()
		// }
		// zlog.Info("structCommandWithMethod:", ctype, hasCommand, argStructType, hasCommand, args)
		if ctype == CommandExecute || ctype == CommandExpand {
			err := zfields.ParseCommandArgsToStructFields(args, argVal)
			if err != nil {
				return err.Error()
			}
		}
		sparams = append(sparams, argVal.Elem())
	}
	vals := method.Func.Call(sparams)
	if len(vals) == 0 {
		return ""
	}
	return vals[0].Interface().(string)
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

func (s *Session) specialMethodNames(structure any) []string {
	var names []string
	rval := reflect.ValueOf(structure)
	t := rval.Type()
	if rval.Kind() == reflect.Pointer {

	}
	// zlog.Info("specialMethodNames:", reflect.TypeOf(structure), t.NumMethod(), rval.Type(), rval.Kind())
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		if mtype.NumIn() == 1 || mtype.In(1) != commandInfoType { // this is 0, so 1 is command-info
			continue
		}
		command := strings.ToLower(method.Name)
		names = append(names, command)
	}
	return names
}

func (s *Session) getChildNodes(where, mode string, forExpand bool) map[string]any {
	return s.getChildNodesOf(s.currentNodeValue(), where, mode, forExpand)
}

func (s *Session) getChildNodesOf(node any, where, mode string, forExpand bool) map[string]any {
	// zlog.Info("getChildNodes:", reflect.TypeOf(node))
	m := map[string]any{}
	for _, st := range anonStructsAndSelf(node) {
		s.addChildNodes(where, mode, forExpand, m, st)
	}
	return m
}

func initCommander(commander any) {
	rval := reflect.ValueOf(commander)
	if rval.Kind() != reflect.Pointer {
		commander = rval.Addr().Interface()
	}
	initer, _ := commander.(Initer)
	// zlog.Info("initCommander:", zlog.Pointer(commander), reflect.TypeOf(commander), initer != nil)
	if initer != nil {
		initer.Init()
	}
}

func (s *Session) addChildNodes(where, mode string, forExpand bool, m map[string]any, parent any) {
	// zlog.Info("s.addChildNode", parent)
	zreflect.ForEachField(parent, zreflect.FlattenIfAnonymous, func(each zreflect.FieldInfo) bool {
		if each.ReflectValue.Kind() == reflect.Pointer {
			each.ReflectValue = each.ReflectValue.Elem()
		}
		if each.ReflectValue.Kind() != reflect.Struct {
			return true
		}
		commander := each.ReflectValue.Addr().Interface()
		meths := s.specialMethodNames(commander)
		if len(meths) == 0 {
			_, no := commander.(NodeOwner)
			_, io := commander.(Initer)
			_, do := commander.(zstr.Describer)
			// zlog.Info("Not node?:", parent, commander, each.StructField.Name, no, io, do, reflect.TypeOf(commander))
			if !no && !io && !do {
				return true // go to next
			}
		}
		name := strings.ToLower(each.StructField.Name)
		// zlog.Info("addChildNode From field", name, commander)
		initCommander(commander)
		m[name] = commander
		return true
	})
	no, _ := parent.(NodeOwner)
	if no != nil {
		cn := no.GetChildrenNodes(s, where, mode, forExpand)
		for n, commander := range cn {
			initCommander(commander)
			m[n] = commander
		}
	}
}

type Help struct {
	Method      string
	Description string
	Args        []zstr.KeyValue
}

func (s *Session) GetAllMethodsHelp(structure any) []Help {
	var help []Help
	rval := reflect.ValueOf(structure)
	t := rval.Type()
	for m := 0; m < t.NumMethod(); m++ {
		var h Help
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
		i++
		var args []reflect.Value
		args = append(args, rval)
		c := &CommandInfo{Session: s, Type: CommandHelp}
		args = append(args, reflect.ValueOf(c))
		for i := 2; i < mtype.NumIn(); i++ {
			atype := mtype.In(i)
			z := reflect.Zero(atype)
			args = append(args, z)
		}
		rets := method.Func.Call(args)
		if len(rets) != 0 {
			h.Description = rets[0].Interface().(string)
		}
		h.Method = strings.ToLower(method.Name)
		if i < mtype.NumIn() {
			atype := mtype.In(i)
			z := reflect.Zero(atype)
			// zlog.Info("Help:", method.Name, i, mtype.NumIn(), hasCommand)
			h.Args = zfields.GetCommandArgsHelpForStructFields(z.Interface())
		}
		help = append(help, h)
	}
	return help
}

func (s *Session) findNodeInPath(sn []namedNode, path string) (nodes []namedNode, err error) {
	var startNodes []namedNode
	zslice.CopyTo(&startNodes, sn)
	forExpand := true
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			slen := len(startNodes)
			if slen <= 1 {
				return nil, errors.New("cd .. below root")
			}
			startNodes = startNodes[:slen-1]
			continue
		}
		// zlog.Info("findNodeInPath:", len(startNodes), len(sn), part, "path:", path)
		nodes := s.getChildNodesOf(startNodes[len(startNodes)-1].node, part, "", forExpand)
		node, got := nodes[part]
		if !got {
			return nil, errors.New("no directory: '" + part + "'")
		}
		nn := namedNode{part, node}
		startNodes = append(startNodes, nn)
	}
	// zlog.Info("findNodeInPath done:", len(startNodes))
	return startNodes, nil
}

func (s *Session) changeDirectory(path string) error {
	// zlog.Info("changeDirectory:", path)
	if path == "" {
		// todo
		return s.changeDirectory("/")
	}
	if path == "/" {
		s.nodeHistory = s.nodeHistory[:1]
		s.updatePrompt()
		return nil
	}
	var nodes []namedNode
	var err error
	if path[0] == '/' {
		hist := []namedNode{s.nodeHistory[0]}
		nodes, err = s.findNodeInPath(hist, path[1:])
	} else {
		nodes, err = s.findNodeInPath(s.nodeHistory, path)
	}
	if err != nil {
		s.TermSession.Writeln(err)
		return err
	}
	s.nodeHistory = nodes
	s.updatePrompt()
	return nil
}

func (s *Session) GotoChildNode(dir string, node any) {
	nn := namedNode{dir, node}
	s.nodeHistory = append(s.nodeHistory, nn)
	s.updatePrompt()
}

func (s *Session) expandChildStubArg(line, command string) (newLine string, newPos int, ok bool) {
	if line == command {
		return line + " ", len(line) + 1, true
	}
	var rest string
	if zstr.HasPrefix(line, command+" ", &rest) {
		return s.expandChildren(rest, command+" ")
	}
	return
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
