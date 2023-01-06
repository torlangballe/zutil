package zcommands

import (
	"reflect"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
)

type Session struct {
	id          string
	path        string
	currentNode any
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
	sessionID := ts.ContextSessionID()
	str := c.HandleLine(line, sessionID)
	if str != "" {
		ts.WriteLine(str)
	}
	return true
}

func (c *Commander) HandleLine(line string, sessionID string) string {
	s := c.sessions[sessionID]
	if s == nil {
		s = new(Session)
		s.id = sessionID
		s.currentNode = c.rootNode
	}
	c.sessions[sessionID] = s
	parts := zstr.GetQuotedArgs(line)
	if len(parts) == 0 {
		return ""
	}
	command := parts[0]
	args := parts[1:]
	switch command {
	case "cd":
		if len(args) < 1 {
			return "cd needs one path argument"
		}
		return s.cd(args[0])
	default:
		return s.structCommand(command, args)
	}
}

func (s *Session) structCommand(command string, args []string) string {
	rval := reflect.ValueOf(s.currentNode)
	t := rval.Type()
	zlog.Info("SC:", t, t.NumMethod())
	// et := t
	// if rval.Kind() == reflect.Ptr {
	// 	et = rval.Elem().Type()
	// }
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		zlog.Info("command:", method, mtype)
	}
	return ""
}

func (s *Session) cd(path string) string {
	return ""
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
