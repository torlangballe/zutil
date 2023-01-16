package zcommands

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zbool"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
	"github.com/torlangballe/zutil/zwords"
)

type ArgType string

const (
	ArgOther  ArgType = ""
	ArgString ArgType = "string"
	ArgInt    ArgType = "int"
	ArgFloat  ArgType = "float"
	ArgBool   ArgType = "bool"
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
}

type Commander struct {
	sessions map[string]*Session
	rootNode any
}

func NewCommander(rootNode any, term *zterm.Terminal) *Commander {
	c := new(Commander)
	c.rootNode = rootNode
	c.sessions = map[string]*Session{}
	term.HandleNewSession = func(ts *zterm.Session) func(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
		s := new(Session)
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
	parts := zstr.GetQuotedArgs(line)
	if len(parts) == 0 {
		return
	}
	command := parts[0]
	args := parts[1:]
	switch command {
	case "cd":
		dir := ""
		if len(args) > 0 {
			dir = args[0]
		}
		s.changeDirectory(dir)
	case "help":
		s.writeHelp(args)
	case "ls":
		s.listDir(args)
	case "pwd":
		s.TermSession.Writeln(s.path())
	default:
		s.structCommand(command, args)
	}
}

func (s *Session) autoComplete(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	line = strings.TrimLeft(line, " ")
	if key == 9 {
		var rest string
		if line == "cd" {
			return line + " ", pos + 1, true
		}
		if zstr.HasPrefix(line, "cd ", &rest) {
			return s.expandChildren(rest, "cd ")
		}
		if strings.Contains(line, " ") {
			return
		}
		return s.expandCommands(line)
	}
	return
}

func (s *Session) expandChildren(fileStub, prefix string) (newLine string, newPos int, ok bool) {
	var names []string
	for n := range s.getChildNodes() {
		names = append(names, n)
	}
	return s.expandForList(fileStub, names, prefix)
}

func (s *Session) allCommands() []string {
	meths := s.methodNames(true)
	meths = append(meths, "cd", "pwd", "ls", "help")
	return meths
}

func (s *Session) expandCommands(stub string) (newLine string, newPos int, ok bool) {
	return s.expandForList(stub, s.allCommands(), "")
}

func (s *Session) expandForList(stub string, list []string, prefix string) (newLine string, newPos int, ok bool) {
	var commands []string
	for _, c := range list {
		if strings.HasPrefix(c, stub) {
			commands = append(commands, c)
		}
	}
	zlog.Info("ExpChiexpandForListldren:", stub, list)

	if len(commands) == 0 {
		return
	}
	if len(commands) == 1 {
		m := prefix + commands[0]
		return m, len(m), true
	}
	s.TermSession.Writeln("\n" + strings.Join(commands, " "))
	stub = zstr.SliceCommonExtremity(commands, true)
	if stub != "" {
		stub = prefix + stub
		zlog.Info("EXList: '"+stub+"'", commands)
		return stub, len(stub), true
	}
	zlog.Info("EXList2: '"+stub+"'", commands)
	return
}

func (s *Session) currentNode() any {
	return s.nodeHistory[len(s.nodeHistory)-1].node
}

func (s *Session) listDir(args []string) {
	nodes := s.getChildNodes()
	for name := range nodes {
		s.TermSession.Writeln(name)
	}
}

func (s *Session) writeHelp(args []string) {
	tabs := tabwriter.NewWriter(s.TermSession.Writer(), 5, 0, 3, ' ', 0)
	for i, n := range s.methodNames(false) { // get ALL methods, so we get correct index to get args
		// zlog.Info("writeHelp", n, s.TermSession != nil)
		if n == "" {
			continue
		}
		fmt.Fprint(tabs, zstr.EscYellow+n, "\t"+zstr.EscCyan)
		help := s.getMethodsHelp(i)
		help = strings.Replace(help, "\t", "\t"+zstr.EscNoColor, 1) + zstr.EscNoColor
		fmt.Fprint(tabs, help)
		fmt.Fprintln(tabs, "")
	}
	tabs.Flush()
}

func (s *Session) path() string {
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
	s.TermSession.SetPrompt(s.path() + "> ")
}

func (s *Session) structCommand(command string, args []string) string {
	meths := s.methodNames(true)
	i := zstr.IndexOf(command, meths)
	if i == -1 {
		s.TermSession.Writeln("command not found:", command)
		return ""
	}
	rval := reflect.ValueOf(s.currentNode())
	t := rval.Type()
	method := t.Method(i)
	mtype := method.Type
	var needed int
	params := []reflect.Value{reflect.ValueOf(s.currentNode()), reflect.ValueOf(s)}
	for i := 2; i < mtype.NumIn(); i++ {
		isPointer := (mtype.In(i).Kind() == reflect.Pointer)
		kind := mtype.In(i).Kind()
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
			continue
		}
		if needed != 0 {
			continue
		}
		arg := zstr.ExtractFirstString(&args)
		k := zreflect.KindFromReflectKind(kind)
		n, err := strconv.ParseFloat(arg, 10)
		if k == zreflect.KindInt || k == zreflect.KindFloat {
			if err != nil {
				s.TermSession.Writeln("error parsing '" + arg + "' to number.")
				return ""
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
				return ""
			}
			av.SetBool(b)
		default:
			s.TermSession.Writeln("unsupported parameter #:", i, av.Kind())
			return ""
		}
		if isPointer {
			av = av.Addr()
		}
		params = append(params, av)
	}
	if needed > 0 {
		s.TermSession.Writeln(zwords.Pluralize("argument", needed), "needed")
		return ""
	}
	if len(args) > 0 {
		s.TermSession.Writeln("extra arguments unused:", args)
	}
	method.Func.Call(params)
	return ""
}

func (s *Session) methodNames(onlyCommands bool) []string {
	var names []string
	c := s.currentNode()
	rval := reflect.ValueOf(c)
	t := rval.Type()
	// et := t
	// if rval.Kind() == reflect.Ptr {
	// 	et = rval.Elem().Type()
	// }
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		if mtype.NumIn() == 0 || reflect.TypeOf(&Session{}) != mtype.In(1) {
			zlog.Info("No Session", method.Name, mtype.NumIn(), mtype.In(1))
			if !onlyCommands {
				names = append(names, "")
			}
			continue
		}
		command := strings.ToLower(method.Name)
		names = append(names, command)
		// zlog.Info("command:", method.Name, mtype)
	}
	return names
}

func (s *Session) getChildNodes() map[string]any {
	m := map[string]any{}
	zreflect.ForEachField(s.currentNode(), func(index int, v reflect.Value, sf reflect.StructField) {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return
		}
		meths := s.methodNames(true)
		if len(meths) == 0 {
			return
		}
		name := strings.ToLower(sf.Name)
		m[name] = v.Interface()
	})
	return m
}

func (s *Session) getMethodsHelp(index int) string {
	t := reflect.ValueOf(s.currentNode()).Type()
	method := t.Method(index)
	mtype := method.Type
	var args []reflect.Value
	args = append(args, reflect.ValueOf(s.currentNode()))
	var session *Session
	args = append(args, reflect.ValueOf(session))
	for i := 2; i < mtype.NumIn(); i++ {
		atype := mtype.In(i)
		z := reflect.Zero(atype)
		args = append(args, z)
	}
	rets := method.Func.Call(args)
	help := rets[0].Interface().(string)
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
	nn := namedNode{dir, node}
	s.nodeHistory = append(s.nodeHistory, nn)
	s.updatePrompt()
}

// func findChildrenNodes(node any) {
// 	zreflect.ForEachField(node, func(index int, val reflect.Value, sf reflect.StructField) {
// 		var column string
// 		dbTags := zreflect.GetTagAsMap(string(sf.Tag))["db"]

// 		rval := reflect.ValueOf(c)
// 		t := rval.Type()
// 		et := t
// 		if rval.Kind() == reflect.Ptr {
// 			et = rval.Elem().Type()
// 		}
// 	})
// }
