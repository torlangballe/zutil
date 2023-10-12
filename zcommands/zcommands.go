//go:build server

package zcommands

import (
	"os/user"
	"reflect"
	"strings"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zreflect"
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
	nodeHistory []namedNode
	commander   *Commander
}

type Commander struct {
	sessions    map[string]*Session
	rootNode    any
	GlobalNodes []any
}

type CommandInfo struct {
	Session *Session
	Type    CommandType
}

type methodNode struct {
	Name string
}

var (
	AllowBash       bool
	AddressIP4      string
	commandInfoType = reflect.TypeOf(&CommandInfo{})
)

func NewCommander(rootNode any, term *zterm.Terminal) *Commander {
	c := new(Commander)
	c.rootNode = rootNode
	c.sessions = map[string]*Session{}
	c.GlobalNodes = []any{&defaultCommands{}}
	term.HandleNewSession = func(ts *zterm.Session) func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		s := new(Session)
		s.commander = c
		s.id = ts.ContextSessionID()
		nn := namedNode{"/", c.rootNode}
		s.nodeHistory = []namedNode{nn}
		s.TermSession = ts
		c.sessions[s.id] = s
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

func (s *Session) doCommand(line string, ctype CommandType) string {
	parts := zstr.GetQuotedArgs(line)
	if len(parts) == 0 {
		return ""
	}
	command := parts[0]
	args := parts[1:]
	// zlog.Info("doCommand", command, args)
	str, got := s.structCommand(s.currentNode(), command, args, ctype)
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
		nodes := append(s.commander.GlobalNodes, s.currentNode())
		for _, n := range nodes {
			names = append(names, s.methodNames(n)...)
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
	for n := range s.getChildNodes() {
		names = append(names, n)
	}
	return s.expandForList(fileStub, names, prefix)
}

func (s *Session) expandForList(stub string, list []string, prefix string) (newLine string, newPos int, ok bool) {
	// zlog.Info("expandForList1", stub, list)
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
	diff := zstr.SliceCommonExtremity(commands, true)
	zstr.HasPrefix(diff, stub, &add)
	if diff != "" {
		zlog.Info("expandForList:", diff, add, stub, prefix, zlog.CallingStackString())
		add = prefix + add
		return add, len(add), true
	}
	return
}

func (s *Session) currentNode() any {
	return s.nodeHistory[len(s.nodeHistory)-1].node
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
	s.TermSession.SetPrompt(s.Path() + "> ")
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
	return vals[0].Interface().(string)
}

func anonStructsAndSelf(structure any) []any {
	anon := []any{structure}
	zreflect.ForEachField(structure, nil, func(index int, v reflect.Value, sf reflect.StructField) bool {
		if sf.Anonymous {
			if v.CanAddr() {
				v = v.Addr()
			}
			anon = append(anon, v.Interface())
		}
		return true
	})
	return anon
}

func (s *Session) methodNames(structure any) []string {
	var names []string
	rval := reflect.ValueOf(structure)
	t := rval.Type()
	if rval.Kind() == reflect.Pointer {

	}
	// zlog.Info("methodNames:", reflect.TypeOf(structure), t.NumMethod(), rval.Type(), rval.Kind())
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		command := strings.ToLower(method.Name)
		names = append(names, command)
	}
	return names
}

func (s *Session) getChildNodes() map[string]any {
	// zlog.Info("getChildNodes:", reflect.TypeOf(s.currentNode()))
	m := map[string]any{}
	for _, st := range anonStructsAndSelf(s.currentNode()) {
		s.addChildNodes(m, st)
	}
	return m
}

func (s *Session) addChildNodes(m map[string]any, parent any) {
	// zlog.Info("AddChildNodes:", reflect.TypeOf(parent))
	zreflect.ForEachField(parent, zreflect.FlattenIfAnonymous, func(index int, v reflect.Value, sf reflect.StructField) bool {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return true
		}
		meths := s.methodNames(v.Addr().Interface())
		if len(meths) == 0 {
			return true
		}
		name := strings.ToLower(sf.Name)
		m[name] = v.Addr().Interface()
		return true
	})
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
		if hasCommand {
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

func (s *Session) changeDirectory(path string) {
	zlog.Info("changeDirectory:", path)
	if path == "" {
		// todo
		return
	}
	if path == ".." {
		nc := len(s.nodeHistory)
		if nc == 0 {
			s.changeDirectory("")
			return
		}
		s.nodeHistory = s.nodeHistory[:nc-1]
		s.updatePrompt()
		return
	}
	parts := strings.SplitN(path, "/", 2)
	dir := parts[0]
	nodes := s.getChildNodes()
	node, got := nodes[dir]
	if !got {
		s.TermSession.Writeln("no directory: '" + dir + "'")
		return
	}
	s.GotoChildNode(dir, node)
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

func CreateCommanderAndTerminal(welcome string, keysdir string, hardUsers map[string]string, rootNode any, port int) *Commander {
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
	for user, pass := range hardUsers {
		terminal.AddHardcodedUser(user, pass)
	}
	commander := NewCommander(rootNode, terminal)
	terminal.HandleLine = commander.HandleTerminalLine
	go terminal.ListenForever(port)
	return commander
}
