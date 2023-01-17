package zcommands

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zstr"
)

type DefaultCommands struct {
}

func (r *DefaultCommands) Cd(s *Session, path *string) string {
	if s == nil {
		return "<path>\tchange directory to path. .. is to parent, - is to previous."
	}
	dir := ""
	if path != nil {
		dir = *path
	}
	s.changeDirectory(dir)
	return ""
}

func (r *DefaultCommands) Help(s *Session) string {
	if s == nil {
		return "\tshows this help."
	}
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
	return ""
}

func (r *DefaultCommands) LS(s *Session, path *string) string {
	if s == nil {
		return "\tlist childrem match path, or all in current directory."
	}
	nodes := s.getChildNodes()
	for name := range nodes {
		s.TermSession.Writeln(name)
	}
	return ""
}

func (r *DefaultCommands) PWD(s *Session, path *string) string {
	if s == nil {
		return "\tPrint Working Directory, show path to where you are in hierarchy."
	}
	s.TermSession.Writeln(s.path())
	return ""
}
