//go:build server

package zcommands

import (
	"fmt"
	"io"
	"runtime/pprof"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zprocess"
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
	// zdevice.InitNetworkBandwidth()
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
	h, _ := c.Session.currentNodeValue().(helpGetter)
	if h != nil {
		str := h.GetHelpForNode_()
		c.Session.TermSession.Writeln(str)
	}
	tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
	helpForStruct(c.Session, c.Session.currentNodeValue(), tabs)
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

func (d *defaultCommands) LS(c *CommandInfo, a struct {
	Where string `zui:"default:,desc:context-specific constraint."`
}) string {
	return BaseGetNodes(c, false, "", a.Where)
}

func (d *defaultCommands) LL(c *CommandInfo, a struct {
	Where string `zui:"default:,desc:context-specific constraint."`
}) string {
	return BaseGetNodes(c, true, "", a.Where)
}

func BaseGetNodes(c *CommandInfo, details bool, mode string, where string) string {
	if c.Type == CommandHelp {
		str := "list nodes. Nodes are like directories, but with commands and content."
		if details {
			str += "\tShows columns of detailed information."
		}
		return str
	}
	if c.Type == CommandExpand {
		return ""
	}
	tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
	tabs.MaxColumnWidth = 60
	forExpand := false
	nodes := c.Session.getChildNodes(where, mode, forExpand)

	var cols []string
	if details {
		for _, n := range nodes {
			cg, _ := n.(ColumnGetter)
			if cg != nil {
				for _, kv := range cg.GetColumns() {
					zstr.AddToSet(&cols, kv.Key)
				}
			}
		}
		zstr.AddToSet(&cols, "desc")
		fmt.Fprint(tabs, zstr.EscGreen)
		fmt.Fprint(tabs, "node\t")
		for _, c := range cols {
			fmt.Fprint(tabs, c, "\t")
		}
		fmt.Fprint(tabs, zstr.EscNoColor, "\n")
	}
	for name, n := range nodes {
		fmt.Fprint(tabs, name, "\t")
		if details {
			var nodeCols []zstr.KeyValue
			cg, _ := n.(ColumnGetter)
			if cg != nil {
				nodeCols = cg.GetColumns()
			}
			for _, c := range cols {
				kv, _ := zstr.KeyValuesFindForKey(nodeCols, c)
				if kv != nil {
					fmt.Fprint(tabs, kv.Value)
					fmt.Fprint(tabs, "\t")
				}
			}
		}
		do, _ := n.(zstr.Describer)
		if do != nil {
			fmt.Fprint(tabs, do.GetDescription())
		}
		fmt.Fprint(tabs, "\n")
	}
	tabs.Flush()
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
	Command string `zui:"desc:text to execute as a bash command line."`
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
	// for _, line := range zdebug.GetProfileCommandLineGetters(AddressIP4) {
	// 	c.Session.TermSession.Writeln(line)
	// }
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
