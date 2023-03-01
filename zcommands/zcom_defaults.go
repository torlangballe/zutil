//go:build server

package zcommands

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
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

func (d *defaultCommands) Cd(c *CommandInfo, path *string) string {
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

func (d *defaultCommands) Help(c *CommandInfo) string {
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

func (d *defaultCommands) LS(c *CommandInfo, path *string) string {
	if c.Type == CommandHelp {
		return "\tlist children match path, or all in current directory."
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

func (d *defaultCommands) PWD(c *CommandInfo, path *string) string {
	if c.Type == CommandHelp {
		return "\tPrint Working Directory, show path to where you are in hierarchy."
	}
	if c.Type == CommandExpand {
		return ""
	}
	c.Session.TermSession.Writeln(c.Session.Path())
	return ""
}

type copier struct {
	s *zterm.Session
}

func (c copier) Read(p []byte) (n int, err error) {
	val, err := c.s.ReadValueLine()
	if err != nil {
		return 0, err
	}
	data := []byte(val)
	max := zint.Min(len(data), len(p))
	copy(p, []byte(val)[:max])
	return max, io.EOF
}

func (d *defaultCommands) Bash(c *CommandInfo, command string) string {
	if c.Type == CommandHelp {
		return "<command> \"[arguments]\"\tCall bash shell command on server."
	}
	if c.Type == CommandExpand {
		return ""
	}
	cmd, outPipe, errPipe, err := zprocess.MakeCommand("/bin/bash", false, nil, []any{"-c", command}...)
	// fmt.Fprintln(s, "Running via ssh")
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return ""
	}
	copier := new(copier)
	copier.s = c.Session.TermSession
	cmd.Stdin = copier
	w := c.Session.TermSession.Writer()
	go io.Copy(w, outPipe)
	go io.Copy(w, errPipe)
	err = cmd.Run()
	if err != nil {
		c.Session.TermSession.Writeln(err)
	}
	// c.Session.TermSession.Terminal.HandleLine = old
	return ""
}
