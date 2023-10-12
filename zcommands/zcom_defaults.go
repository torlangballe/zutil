//go:build server

package zcommands

import (
	"fmt"
	"io"
	"runtime/pprof"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
	"github.com/torlangballe/zutil/zwords"
)

type defaultCommands struct {
}

type UtilCommands struct {
}

type helpGetter interface {
	GetHelpForNode_() string
}

func init() {
	zdevice.InitNetworkBandwidth()
}

func expandPath(s *Session, path *string) string {
	stub := ""
	if path != nil {
		stub = *path
	}
	return s.ExpandChildren(stub, "")
}

func (d *defaultCommands) Cd(c *CommandInfo, a struct {
	Path string `zui:"allowempty,desc:sub-directory to change to. Use .. to go to parent. - to go to previous directory."`
}) string {
	if c.Type == CommandHelp {
		return "change directory."
	}
	if c.Type == CommandExpand {
		return expandPath(c.Session, &a.Path)
	}
	zlog.Info("CD", a.Path, c.Type)
	c.Session.changeDirectory(a.Path)
	return ""
}

func (d *defaultCommands) Help(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "shows this help."
	}
	if c.Type == CommandExpand {
		return ""
	}
	h, _ := c.Session.currentNode().(helpGetter)
	if h != nil {
		str := h.GetHelpForNode_()
		c.Session.TermSession.Writeln(str)
	}
	tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
	helpForStruct(c.Session, c.Session.currentNode(), tabs)
	for _, n := range c.Session.commander.GlobalNodes {
		helpForStruct(c.Session, n, tabs)
	}
	tabs.Flush()
	return ""
}

func helpForStruct(s *Session, structure any, tabs *zstr.TabWriter) {
	for _, h := range s.GetAllMethodsHelp(structure) {
		fmt.Fprint(tabs, zstr.EscYellow, h.Method, "\t")
		fmt.Fprint(tabs, zstr.EscNoColor, h.Description, zstr.EscNoColor, "\n")

		for _, arg := range h.Args {
			fmt.Fprint(tabs, zstr.EscCyan, "  ", arg.Key, "\t")
			fmt.Fprint(tabs, zstr.EscNoColor, arg.Value, zstr.EscNoColor, "\n")
		}
	}
}

func (d *defaultCommands) LS(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "list nodes. Nodes are like directories, but with commands and content."
	}
	if c.Type == CommandExpand {
		return ""
	}
	nodes := c.Session.getChildNodes()
	for name := range nodes {
		c.Session.TermSession.Writeln(name)
	}
	return ""
}

func (d *defaultCommands) PWD(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "Print Working Directory, show path to where you are in the node hierarchy."
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

func (d *UtilCommands) Bash(c *CommandInfo, a struct {
	Command string `zui:"desc:text to execute as a bash command line"`
}) string {
	if c.Type == CommandHelp {
		return "all bash shell command on server."
	}
	if c.Type == CommandExpand {
		return ""
	}
	if !AllowBash {
		c.Session.TermSession.Writeln("bash not enabled.")
		return ""
	}
	cmd, outPipe, errPipe, err := zprocess.MakeCommand("/bin/bash", nil, false, nil, []any{"-c", a.Command}...)
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

func (d *UtilCommands) GOs(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "show all goroutines"
	}
	if c.Type == CommandExpand {
		return ""
	}
	pprof.Lookup("goroutine").WriteTo(c.Session.TermSession.Writer(), 1)
	return ""
}

func (d *UtilCommands) Debug(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "show commands to profile this programpro"
	}
	if c.Type == CommandExpand {
		return ""
	}
	for _, n := range []string{"heap", "profile", "block", "mutex"} {
		str := fmt.Sprintf("curl http://%s:%d/debug/pprof/%s > ~/%s && go tool pprof -web ~/%s",
			AddressIP4, zrest.ProfilingPort, n, n, n)
		c.Session.TermSession.Writeln(str)
	}
	return ""
}

func (d *UtilCommands) Net(c *CommandInfo) string {
	if c.Type == CommandHelp {
		return "Show i/o network bandwidth per second, and drops/sec and errors/sec."
	}
	if c.Type == CommandExpand {
		return ""
	}
	nets, err := zdevice.NetworkBandwidthPerSec()
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return ""
	}
	tabs := tabwriter.NewWriter(c.Session.TermSession.Writer(), 5, 0, 3, ' ', 0)
	fmt.Fprintln(tabs, zstr.EscGreen+"name\treceived\tsent\tdrops\terrors"+zstr.EscNoColor)
	names := zstr.SortedMapKeys(nets)
	for _, name := range names {
		info := nets[name]
		if info.In.Bytes == 0 && info.Out.Bytes == 0 {
			continue
		}
		fmt.Fprint(tabs, zstr.EscCyan, name, "\t")
		fmt.Fprint(tabs, zwords.GetBandwidthString(info.In.Bytes, "", 2)+"/s", "\t")
		fmt.Fprint(tabs, zwords.GetBandwidthString(info.Out.Bytes, "", 2)+"/s", "\t")
		fmt.Fprint(tabs, info.In.Drops+info.Out.Drops, "\t")
		fmt.Fprint(tabs, info.In.Errors+info.Out.Errors, zstr.EscNoColor, "\n")
	}
	tabs.Flush()
	return ""
}
