//go:build server

package zcommands

import (
	"os/user"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
	"github.com/torlangballe/zutil/zwords"
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
	s.doCommand(line, CommandExecute)
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
			if str == "" {
				return
			}
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
	zlog.Info("expandForList", stub, list)
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
	stub = zstr.SliceCommonExtremity(commands, true)
	if stub != "" {
		stub = prefix + stub
		return stub, len(stub), true
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
	// zlog.Info("structCommand", command, args, ctype)
	for _, st := range anonStructsAndSelf(structure) {
		rval := reflect.ValueOf(st)
		t := rval.Type()
		for m := 0; m < t.NumMethod(); m++ {
			method := t.Method(m)
			mtype := method.Type
			// zlog.Info("MethNames:", method.Name, mtype.NumIn())
			if mtype.NumIn() == 1 || mtype.In(1) != commandInfoType {
				continue
			}
			if strings.ToLower(method.Name) == command {
				return s.structCommandWithMethod(method, rval, args, ctype)
			}
		}
	}
	return
}

func (s *Session) structCommandWithMethod(method reflect.Method, structVal reflect.Value, args []string, ctype CommandType) (result string, got bool) {
	mtype := method.Type
	var needed int
	c := &CommandInfo{Session: s, Type: ctype}
	params := []reflect.Value{structVal, reflect.ValueOf(c)}
	for i := 2; i < mtype.NumIn(); i++ {
		kind := mtype.In(i).Kind()
		isPointer := (kind == reflect.Pointer)
		av := reflect.New(mtype.In(i)).Elem()
		if isPointer {
			kind = mtype.In(i).Elem().Kind()
			av = reflect.New(mtype.In(i).Elem()).Elem()
		} else {
		}
		if len(args) == 0 {
			if isPointer {
				av = reflect.New(mtype.In(i)).Elem()
			} else {
				needed++
				continue
			}
			params = append(params, av)
			// zlog.Info("structCommand param", av)
			continue
		}
		if needed != 0 {
			continue
		}
		arg := zstr.ExtractFirstString(&args)
		k := zreflect.KindFromReflectKindAndType(kind, mtype.In(i))
		n, err := strconv.ParseFloat(arg, 10)
		if k == zreflect.KindInt || k == zreflect.KindFloat {
			if err != nil {
				s.TermSession.Writeln("error parsing '" + arg + "' to number.")
				return "", true // we return got==true
			}
		}
		switch kind {
		case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int8:
			av.SetInt(int64(n))
		case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint8:
			av.SetUint(uint64(n))
		case reflect.Float32, reflect.Float64:
			av.SetFloat(n)
		case reflect.String:
			av.SetString(arg)
		case reflect.Bool:
			b, err := zbool.FromStringWithError(arg)
			if err != nil {
				s.TermSession.Writeln(arg+":", err)
				return "", true // we return got==true
			}
			av.SetBool(b)
		default:
			s.TermSession.Writeln("unsupported parameter #:", i, av.Kind())
			return "", true // we return got==true
		}
		if isPointer {
			av = av.Addr()
		}
		params = append(params, av)
	}
	if needed > 0 {
		s.TermSession.Writeln(zwords.Pluralize("argument", needed), "needed")
		return "", true // we return got==true
	}
	if len(args) > 0 {
		s.TermSession.Writeln("extra arguments unused:", args)
	}
	// zlog.Info("Call:", method.Name, len(params), params)
	vals := method.Func.Call(params)
	return vals[0].Interface().(string), true
}

func anonStructsAndSelf(structure any) []any {
	anon := []any{structure}
	zreflect.ForEachField(structure, false, func(index int, v reflect.Value, sf reflect.StructField) bool {
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
		mtype := method.Type
		// zlog.Info("MethNames:", method.Name, mtype.NumIn())
		if mtype.NumIn() == 1 || mtype.In(1) != commandInfoType {
			// zlog.Info("No Session", method.Name, mtype.NumIn(), mtype.In(1))
			continue
		}
		command := strings.ToLower(method.Name)
		names = append(names, command)
		// zlog.Info("command:", method.Name, mtype)
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
	zreflect.ForEachField(parent, true, func(index int, v reflect.Value, sf reflect.StructField) bool {
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
	Args        string
	Description string
}

func (s *Session) GetAllMethodsHelp(structure any) []Help {
	var help []Help
	rval := reflect.ValueOf(structure)
	t := rval.Type()
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		if mtype.NumIn() == 1 || mtype.In(1) != commandInfoType {
			// zlog.Info("No Session", method.Name, mtype.NumIn(), mtype.In(1))
			continue
		}
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
		returnString := rets[0].Interface().(string)
		var h Help
		h.Method = strings.ToLower(method.Name)
		zstr.SplitN(returnString, "\t", &h.Args, &h.Description)
		help = append(help, h)
	}
	return help
}

func (s *Session) changeDirectory(path string) {
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
