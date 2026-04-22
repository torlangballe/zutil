package zcommands

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zslices"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
)

type Session struct {
	id               string
	TermSession      *zterm.Session
	promptExtra      string
	NodeHistory      []Node
	NodeNumberedList map[int]Node
	commander        *Commander
	executor         *znamedfuncs.Executor
}

func (s *Session) SetNumberedNode(node Node, i int) {
	s.NodeNumberedList[i] = node
}

func (s *Session) PathAsItsTopNode(path string, nodeTypes NodeType) (Node, error) {
	nodes, err := s.PathAsNodes(path, nodeTypes)
	if err != nil {
		return Node{}, err
	}
	return nodes[len(nodes)-1], nil
}

func (s *Session) PathAsNodes(path string, nodeTypes NodeType) ([]Node, error) {
	var startNodes []Node

	n, _ := strconv.Atoi(path)
	if n > 0 {
		node, ok := s.NodeNumberedList[n-1]
		if !ok {
			zlog.Info("BadList:", n, zmap.Keys(s.NodeNumberedList))
			return nil, zlog.NewError("Node number outside last list")
		}
		s.updatePrompt()
		return append(s.NodeHistory, node), nil
	}

	if path[0] == '/' {
		hist := []Node{s.NodeHistory[0]}
		startNodes = hist
		path = path[1:]
	} else {
		startNodes = s.NodeHistory
	}
	for _, part := range strings.Split(path, "/") {
		if part == ".." {
			slen := len(startNodes)
			if slen <= 1 {
				return nil, errors.New("cd .. below root")
			}
			startNodes = startNodes[:slen-1]
			continue
		}
		// zlog.Info("PathAsNodes:", part, reflect.TypeOf(startNodes[len(startNodes)-1].Instance))
		longList := false
		childNodes := NodesForStruct(s, startNodes[len(startNodes)-1].Instance, part, nodeTypes, longList)
		n, _ := zslices.FindFunc(childNodes, func(n Node) bool { // maybe not needed, if part above filters out?
			return n.Name == part
		})
		if n == nil {
			return nil, errors.New("Path part not found")
		}
		startNodes = append(startNodes, *n)
	}
	return startNodes, nil
}

func (s *Session) changeDirectory(path string) error {
	if path == "" {
		// todo
		s.NodeNumberedList = map[int]Node{}
		return s.changeDirectory("/")
	}
	if path == "/" {
		s.NodeHistory = s.NodeHistory[:1]
		s.NodeNumberedList = map[int]Node{}
		s.updatePrompt()
		return nil
	}
	nodes, err := s.PathAsNodes(path, RowNode|ComNode)
	if err != nil {
		s.TermSession.Writeln(err)
		return err
	}
	if len(nodes) == 0 {
		return nil
	}
	if nodes[len(nodes)-1].Type&(VariableNode|FieldNode) != 0 {
		err := errors.New("Path does not lead to a node that can be cd'ed into")
		s.TermSession.Writeln(err)
		return err
	}
	s.NodeHistory = nodes
	s.updatePrompt()
	return nil
}

func (s *Session) GotoChildNode(dir string, node any) {
	nn := MakeNode(dir, ComNode, node, 0)
	s.NodeHistory = append(s.NodeHistory, nn)
	s.updatePrompt()
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
		// zlog.Info("specialMethodNames:", method.Name, mtype.NumIn(), commandInfoType)
		command := strings.ToLower(method.Name)
		names = append(names, command)
	}
	return names
}

func (s *Session) SetPromptExtra(extra string) {
	s.promptExtra = extra
	s.updatePrompt()
}

func (s *Session) TopNode() *Node {
	zlog.Assert(len(s.NodeHistory) != 0)
	return &s.NodeHistory[len(s.NodeHistory)-1]
}

func (s *Session) GetCurrentAndGlobalNodeValues() []any {
	nodes := []any{s.CurrentNodeValue()}
	nodes = append(nodes, zslices.Reversed(s.commander.GlobalComs)...)
	return nodes
}

func (s *Session) doCommand(line string, isExpand bool) string {
	parts := zstr.GetQuotedArgs(line)
	if len(parts) == 0 {
		return ""
	}
	for i, p := range parts {
		rep, err := ReplaceVariablesForInstance(s, p, s.TermSession.UserID())
		if err == nil {
			parts[i] = rep
		} else if !isExpand {
			s.TermSession.Writeln("Error replacing variables:", err)
		}
	}
	command := parts[0]
	args := parts[1:]
	// zlog.Info("doCommand", command, args, isExpand)

	for _, n := range s.GetCurrentAndGlobalNodeValues() {
		if isExpand {
			var add string
			if s.structExpand(n, command, strings.Join(args, " "), &add) {
				return add
			}
		} else {
			if s.structCommand(n, command, args) {
				return ""
			}
		}
		// zlog.Info("doCommand", command, method, got, str)
	}
	s.TermSession.Writeln("command not found:", command)
	return ""
}

func (s *Session) autoComplete(line string, pos int, key rune) (newLine string, newPos int, ok bool) {
	line = strings.TrimLeft(line, " ")
	if key == 9 {
		if strings.Contains(line, " ") {
			str := s.doCommand(line, true)
			// if str == "" {
			// 	return
			// }
			ret := line + str
			return ret, len(ret), true
		}
		var names []string
		coms := append(s.commander.GlobalComs, s.CurrentNodeValue())

		for _, n := range coms {
			names = append(names, s.specialMethodNames(n)...)
		}
		longList := false
		mNodes := NodesForStruct(s, s.CurrentNodeValue(), "", MethodNode, longList)
		for _, n := range mNodes {
			names = append(names, n.Name)
		}
		return s.expandForList(line, names, line)
	}
	return
}

func (s *Session) ExpandChildren(fileStub, addPrefixAfter string, nodeTypes NodeType) (addStub string) {
	n, _, _ := s.expandChildren(fileStub, addPrefixAfter, nodeTypes)
	return n
}

func (s *Session) expandChildren(fileStub, addPrefixAfter string, nodeTypes NodeType) (newLine string, newPos int, ok bool) {
	var names []string

	var dirs string
	stub := zstr.TailUntilWithRest(fileStub, "/", &dirs)
	// if dirs == "" {
	// 	dirs = strings.TrimLeft(fileStub, "/")
	// 	stub = ""
	// }
	stub = strings.TrimRight(stub, "/")

	// zlog.Info("expandChildren:", fileStub, "stub:", stub, "dirs:", dirs)
	var top any
	if dirs == "" {
		top = s.CurrentNodeValue()
		if zstr.HasPrefix(fileStub, "/", &stub) {
			top = s.NodeHistory[0].Instance
		}
	} else {
		nodes, _ := s.PathAsNodes(dirs, RowNode|ComNode)
		top = s.CurrentNodeValue()
		if len(nodes) > 0 {
			if len(nodes) != 0 {
				top = nodes[len(nodes)-1].Instance
			}
		}
	}
	dirs = strings.TrimLeft(dirs, "/")
	longList := false
	for _, n := range NodesForStruct(s, top, "", nodeTypes, longList) {
		names = append(names, n.Name) // zstr.Concat("/", dirs,
	}
	return s.expandForList(stub, names, addPrefixAfter)
}

func (s *Session) expandForList(stub string, list []string, addPrefixAfter string) (newLine string, newPos int, ok bool) {
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
		c = strings.Replace(c, " ", "\\ ", -1)
		m := addPrefixAfter + c
		return m, len(m), true
	}
	s.TermSession.Writeln("\n" + strings.Join(commands, " "))
	var add string
	diff := zstr.CommonExtremityOfSlice(commands, true)
	zstr.HasPrefix(diff, stub, &add)
	if diff != "" {
		add = strings.Replace(add, " ", "\\ ", -1)
		add = addPrefixAfter + add
		return add, len(add), true
	}
	return
}

func (s *Session) CurrentNodeValue() any {
	return s.currentNode().Instance
}

func (s *Session) currentNode() Node {
	return s.NodeHistory[len(s.NodeHistory)-1]
}

func (s *Session) Path() string {
	var str string
	for _, n := range s.NodeHistory {
		if str != "" && str != "/" {
			str += "/"
		}
		str += n.Name
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

func (s *Session) structCommand(structure any, command string, args []string) bool {
	if zreflect.NonPointerKind(structure) != reflect.Struct {
		// str := fmt.Sprint("Value:", zstr.EscGreen, structure, zstr.EscNoColor)
		// s.TermSession.Writeln(str)
		return false
	}
	method, got := findMethodByName(structure, "Command_"+command)
	if got {
		s.structCommandWithMethod(method, reflect.ValueOf(structure), args)
		return true
	}
	return false
}

func (s *Session) structExpand(structure any, command, line string, add *string) bool {
	method, got := findMethodByName(structure, "Expand_"+command)
	if got {
		s.structExpandWithMethod(method, reflect.ValueOf(structure), line, add)
		return true
	}
	return false
}

func (s *Session) structExpandWithMethod(method reflect.Method, structVal reflect.Value, line string, add *string) {
	mtype := method.Type
	if mtype.NumIn() != 4 || mtype.In(1) != commandInfoType {
		*add = zstr.Spaced(": bad method for expand", method.Name, mtype.NumIn(), mtype.In(1))
	}
	c := &CommandInfo{Session: s}
	sparams := []reflect.Value{structVal, reflect.ValueOf(c), reflect.ValueOf(line), reflect.ValueOf(add)}
	method.Func.Call(sparams)
}

func (s *Session) structCommandWithMethod(method reflect.Method, structVal reflect.Value, args []string) {
	mtype := method.Type
	if mtype.NumIn() != 3 || mtype.In(1) != commandInfoType {
		s.TermSession.Writeln("bad method for run command")
		return
	}
	c := &CommandInfo{Session: s}
	argStructType := mtype.In(2)
	argVal := reflect.New(argStructType)
	err := zfields.ParseCommandArgsToStructFields(args, argVal)
	if err != nil {
		s.TermSession.Writeln(err.Error())
		return
	}
	// zlog.Info("structCommandWithMethod:", method.Name, argVal.Type(), argVal.Type())
	sparams := []reflect.Value{structVal, reflect.ValueOf(c), argVal.Elem()}
	method.Func.Call(sparams)
}

func ReplaceVariablesForInstance(s *Session, inString string, uid int64) (string, error) {
	var outErr error
	// zlog.Info("ReplaceVariablesForInstance: inString:", inString)
	out := zstr.ReplaceAllCapturesFunc(zstr.DollarArgWithSpaceRegex, inString, zstr.RegWithoutMatch, func(cap string, index int) string {
		node, err := s.PathAsItsTopNode(cap, VariableNode|FieldNode|StaticFieldNode)
		if err != nil {
			outErr = zlog.Error("Error finding variable for capture $"+cap, ":", err)
			return ""
		}
		if node.Type&(FieldNode|StaticFieldNode) != 0 {
			return node.FieldSVal
		}
		if node.Type == VariableNode {
			vs, is := node.Instance.(fmt.Stringer)
			if !is {
				outErr = zlog.Error("Error casting instance to Variable for capture $" + cap)
				return ""
			}
			// zlog.Info("ReplaceVariablesForInstance capture:", node.Name, node.Instance)
			return vs.String()
		}
		zlog.Info("ReplaceVariablesForInstance: Replacing wrong node type:", cap, node.Type)
		return cap
	})
	return out, outErr
}
