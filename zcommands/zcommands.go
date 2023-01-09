package zcommands

import (
	"fmt"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
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

func NewCommander(rootNode any) *Commander {
	c := new(Commander)
	c.rootNode = rootNode
	c.sessions = map[string]*Session{}
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
	if s == nil {
		s = new(Session)
		s.id = sessionID
		nn := namedNode{"/", c.rootNode}
		s.nodeHistory = []namedNode{nn}
		s.TermSession = ts
		c.sessions[sessionID] = s
		s.updatePrompt()
	}
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
	for i, n := range s.getMethodNames(false) { // get ALL methods, so we get correct index to get args
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
	rval := reflect.ValueOf(s.currentNode())
	t := rval.Type()
	// zlog.Info("SC:", t, t.NumMethod())
	// et := t
	// if rval.Kind() == reflect.Ptr {
	// 	et = rval.Elem().Type()
	// }
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		zlog.Info("command2:", method, mtype)
	}
	return ""
}

func (s *Session) getMethodNames(onlyCommands bool) []string {
	var names []string
	rval := reflect.ValueOf(s.currentNode())
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
	zlog.Info("getChildNodes:", reflect.ValueOf(s.currentNode()).Type())

	zreflect.ForEachField(s.currentNode(), func(index int, v reflect.Value, sf reflect.StructField) {
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return
		}
		meths := s.getMethodNames(true)
		zlog.Info("getChildNodes:", sf.Name, meths)
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
