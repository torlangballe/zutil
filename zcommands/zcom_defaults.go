package zcommands

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zstr"
)

type defaultCommands struct {
}

type helpGetter interface {
	GetHelpForNode() string
}

func expandPath(s *Session, path *string) string {
	stub := ""
	if path != nil {
		stub = *path
	}
	return s.ExpandChildren(stub, "")
}

func (r *defaultCommands) Cd(c *CommandInfo, path *string) string {
	if c.Type == CommandHelp {
		return "<path>\tchange directory to path. .. is to parent, - is to previous."
	}
	if c.Type == CommandExpand {
		return expandPath(c.Session, path)
	}
	dir := ""
	if path != nil {
		dir = *path
	}
	c.Session.changeDirectory(dir)
	return ""
}

func (r *defaultCommands) Help(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "\tshows this help."
	}
	if c.Type == CommandExpand {
		return ""
	}
	h, _ := c.Session.currentNode().(helpGetter)
	if h != nil {
		str := h.GetHelpForNode()
		c.Session.TermSession.Writeln(str)
	}
	tabs := tabwriter.NewWriter(c.Session.TermSession.Writer(), 5, 0, 3, ' ', 0)
	helpForStruct(c.Session, c.Session.currentNode(), tabs)
	for _, n := range c.Session.commander.GlobalNodes {
		helpForStruct(c.Session, n, tabs)
	}
	tabs.Flush()
	return ""
}

func helpForStruct(s *Session, structure any, tabs *tabwriter.Writer) {
	for _, h := range s.GetAllMethodsHelp(structure) {
		fmt.Fprint(tabs, zstr.EscYellow, h.Method, " ")
		fmt.Fprint(tabs, zstr.EscCyan, h.Args, "\t")
		parts := strings.Split(h.Description, "\n")
		if len(parts) == 1 {
			fmt.Fprint(tabs, zstr.EscNoColor, h.Description, "\n")
			continue
		}
		fmt.Fprint(tabs, zstr.EscNoColor, parts[0], "\n")
		for _, part := range parts[1:] {
			fmt.Fprint(tabs, zstr.EscYellow, " ")
			fmt.Fprint(tabs, zstr.EscCyan, " \t")
			fmt.Fprint(tabs, zstr.EscNoColor, part, "\n")
		}
	}
}

func (r *defaultCommands) LS(c *CommandInfo, path *string) string {
	if c.Type == CommandHelp {
		return "\tlist childrem match path, or all in current directory."
	}
	if c.Type == CommandExpand {
		return expandPath(c.Session, path)
	}
	nodes := c.Session.getChildNodes()
	for name := range nodes {
		c.Session.TermSession.Writeln(name)
	}
	return ""
}

func (r *defaultCommands) PWD(c *CommandInfo, path *string) string {
	if c.Type == CommandHelp {
		return "\tPrint Working Directory, show path to where you are in hierarchy."
	}
	if c.Type == CommandExpand {
		return ""
	}
	c.Session.TermSession.Writeln(c.Session.Path())
	return ""
}
