//go:build server

package zcommands

import (
	"fmt"
	"io"
	"reflect"
	"runtime/pprof"
	"strings"
	"text/tabwriter"

	"github.com/torlangballe/zutil/zdevice"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmath"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zterm"
	"github.com/torlangballe/zutil/zwords"
)

type defaultCommands struct{}

type UtilCommands struct {
}

type helpGetter interface {
	GetHelpForNode_() string
}

func init() {
	// zdevice.InitNetworkBandwidth()
}

func ExpandPath(s *Session, line string, add *string, nodeTypes NodeType) {
	last := zstr.TailUntil(line, " ")
	str := s.ExpandChildren(last, "", nodeTypes)
	*add = str
}

func (defaultCommands) Expand_cd(c *CommandInfo, line string, add *string) {
	ExpandPath(c.Session, line, add, RowNode|ComNode)
	// zlog.Info("Expand_cd2", line, *add)
}

func (defaultCommands) Command_cd(c *CommandInfo, a struct {
	Path        string `zui:"allowempty,desc:sub-directory to change to. Use .. to go to parent. - to go to previous directory."`
	Description string `zui:"desc:Change directory. Use .. to go to parent, - to go to previous directory."`
}) {
	c.Session.changeDirectory(a.Path)
}

func (defaultCommands) Expand_edit(c *CommandInfo, line string, add *string) {
	ExpandPath(c.Session, line, add, FieldNode)
	// zlog.Info("Expand_cd2", line, *add)
}

func (defaultCommands) Command_edit(c *CommandInfo, a struct {
	Path        string
	Description string `zui:"desc:Edit a field. Use path to specify name or number of listed items."`
}) {
	nodes, err := c.Session.PathAsNodes(a.Path, FieldNode)
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
	}
	if len(nodes) == 0 {
		c.Session.TermSession.Writeln("No nodes found for path:", a.Path)
		return
	}
	node := nodes[len(nodes)-1]
	if node.Type == VariableNode {
		eo, _ := node.Instance.(Editer)
		if eo != nil {
			eo.Edit(c.Session)
			callUpdater(c, node)
			return
		}
	}
	if node.Type != FieldNode {
		c.Session.TermSession.Writeln("Node is not a field:", a.Path)
		zlog.Info("Node is not a field:", a.Path, zlog.Full(node))
		return
	}
	editNode(c, node)
}

func (defaultCommands) Help(c *CommandInfo) {
	h, _ := c.Session.CurrentNodeValue().(helpGetter)
	if h != nil {
		str := h.GetHelpForNode_()
		c.Session.TermSession.Writeln(str)
	}
	tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
	// helpForStruct(c.Session, c.Session.CurrentNodeValue(), tabs)
	// for _, n := range c.Session.commander.GlobalNodes {
	// 	helpForStruct(c.Session, n, tabs)
	// }
	tabs.Flush()
}

// func helpForStruct(s *Session, structure any, tabs *zstr.TabWriter) {
// 	for _, h := range s.GetAllMethodsHelp(structure) {
// 		fmt.Fprint(tabs, zstr.EscYellow, h.Method, "\t")
// 		fmt.Fprint(tabs, zstr.EscNoColor, h.Description, zstr.EscNoColor, "\n")
// 		for _, arg := range h.Args {
// 			fmt.Fprint(tabs, zstr.EscCyan, "  ", arg.Key, "\t")
// 			fmt.Fprint(tabs, zstr.EscNoColor, arg.Value, zstr.EscNoColor, "\n")
// 		}
// 	}
// }

func (defaultCommands) Expand_ll(c *CommandInfo, line string, add *string) {
	ExpandPath(c.Session, line, add, RowNode|ComNode|FieldNode|MethodNode)
}

func (defaultCommands) Expand_ls(c *CommandInfo, line string, add *string) {
	ExpandPath(c.Session, line, add, RowNode|ComNode|FieldNode|MethodNode)
}

func (defaultCommands) Command_ls(c *CommandInfo, a struct {
	Path string `zui:"default:,desc:context-specific constraint."`
}) {
	listNodes(c, false)
}

func listNodes(c *CommandInfo, longList bool) {
	c.Session.NodeNumberedList = map[int]Node{}
	nodes := NodesForStruct(c.Session, c.Session.CurrentNodeValue(), "", FieldNode|StaticFieldNode|RowNode|ComNode|MethodNode|VariableNode, longList) // use part with **?? or filter below?

	headers := []string{"#", "name"}
	var hasValue bool
	var dotHeaders []string

	for _, n := range nodes {
		if n.Type == MethodNode {
			continue
		}
		var cols zdict.Items
		if n.FieldSVal != "" {
			hasValue = true
			zstr.AddToSet(&headers, "value")
		}
		cg, _ := n.Instance.(ColumnOwner)
		if cg != nil {
			cols = cg.CommandColumns()
		}
		for _, c := range cols {
			if c.Name[0] == '.' {
				if !longList {
					continue
				}
				zstr.AddToSet(&dotHeaders, strings.TrimLeft(c.Name, "."))
				continue
			}
			zstr.AddToSet(&headers, c.Name)
		}
		if n.Description != "" {
			zstr.AddToSet(&headers, "desc")
		}
	}
	headers = append(headers, dotHeaders...)
	methodNodes, nodes := zslice.SplitFunc(nodes, func(n Node) bool {
		return n.Type == MethodNode
	})
	if len(methodNodes) > 0 {
		tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
		// tabs.MaxColumnWidth = 60
		for _, n := range methodNodes {
			fmt.Fprint(tabs, zstr.EscMagenta, n.Name+"*", "\t", zstr.EscWhite)
			fmt.Fprint(tabs, zstr.EscWhite, n.Description, zstr.EscNoColor, "\n")
		}
		tabs.Flush()
	}
	i := 0
	tabs := zstr.NewTabWriter(c.Session.TermSession.Writer())
	tabs.MaxColumnWidth = 60
	if len(nodes) > 0 {
		fmt.Fprint(tabs, zstr.EscGreen, strings.Join(headers, "\t"), zstr.EscNoColor, "\n")
	}
	headers = headers[2:] // skip # and name

	tree := zmath.MakeTree(nodes)
	// zlog.Info("listNodes:", len(nodes), len(tree))
	for _, t := range tree {
		n := t.Instance
		// zlog.Info("List:", n.Name, n.Type, "id:", n.Identifier(), t.Level)
		name := strings.TrimLeft(n.Name, ".")
		if t.Level > 0 {
			name = "⤷" + strings.Repeat(" ", t.Level) + name // ┗━
		}
		col := zstr.EscYellow
		sindex := ""
		if n.Type&VariableNode != 0 {
			name = "⊙ " + name
			col = zstr.EscCyan
		}
		if n.Type&FieldNode != 0 {
			name = "⊙ " + name
		}
		if n.Type&StaticFieldNode != 0 {
			name = "⊘ " + name
		} else {
			c.Session.NodeNumberedList[i] = n
			sindex = fmt.Sprint(i + 1)
			i++
		}
		if n.Type&RowNode != 0 {
			name = "≣ " + name
		}
		if n.Type&(ComNode) != 0 {
			name += "/"
		}
		if n.Type&EnumNode != 0 {
			col = zstr.EscCyan
			name = "◉" + name
		}
		if n.Type == 0 {
			zlog.Info("ls: Unknown node type:", n.Type, ComNode)
			name += "!"
		}
		fmt.Fprint(tabs, zstr.EscWhite, sindex, "\t", col, name+zstr.EscWhite)
		var cols zdict.Items
		cg, _ := n.Instance.(ColumnOwner)
		if cg != nil {
			cols = cg.CommandColumns()
		}
		if hasValue && n.FieldSVal != "" {
			cols.AddToSet("value", n.FieldSVal)
		}
		for _, h := range headers {
			f, _ := zslice.FindFunc(cols, func(item zdict.Item) bool {
				return strings.TrimLeft(item.Name, ".") == h
			})
			val := ""
			if f != nil {
				val = fmt.Sprint(f.Value)
			}
			fmt.Fprint(tabs, "\t", val)
		}
		fmt.Fprint(tabs, zstr.EscNoColor, "\n")
	}
	tabs.Flush()
}

func (defaultCommands) Command_ll(c *CommandInfo, a struct {
	Path string `zui:"default:,desc:context-specific constraint."`
}) {
	listNodes(c, true)
}

func (defaultCommands) Command_pwd(c *CommandInfo, a struct {
	Description string `zui:"desc:Print working directory, show path to where you are in the node hierarchy."`
}) {
	c.Session.TermSession.Writeln(c.Session.Path())
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

func (d *UtilCommands) Command_bash(c *CommandInfo, a struct {
	Command     string `zui:"desc:text to execute as a bash command line."`
	Description string `zui:"desc:Execute a bash command on the server. Output will be printed to terminal. Use with caution."`
}) {
	if !AllowBash {
		c.Session.TermSession.Writeln("bash not enabled.")
	}
	cmd, outPipe, errPipe, err := zprocess.MakeCommand("/bin/bash", nil, false, nil, []any{"-c", a.Command}...)
	// fmt.Fprintln(s, "Running via ssh")
	if err != nil {
		c.Session.TermSession.Writeln(err)
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
}

func (d *UtilCommands) Command_gos(c *CommandInfo, a struct {
	Description string `zui:"desc:Show all goroutines. Use with caution."`
}) {
	pprof.Lookup("goroutine").WriteTo(c.Session.TermSession.Writer(), 1)
}

func (d *UtilCommands) Command_debug(c *CommandInfo, a struct {
	Description string `zui:"desc:Show commands to profile this program."`
}) {
	// for _, line := range zdebug.GetProfileCommandLineGetters(AddressIP4) {
	// 	c.Session.TermSession.Writeln(line)
	// }
}

func (d *UtilCommands) Command_net(c *CommandInfo, a struct {
	Description string `zui:"desc:Show i/o network bandwidth per second, and drops/sec and errors/sec."`
}) {
	nets, err := zdevice.NetworkBandwidthPerSec()
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
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
		fmt.Fprint(tabs, zwords.GetBandwidthString(info.In.Bytes*8, "", 2)+"/s", "\t")
		fmt.Fprint(tabs, zwords.GetBandwidthString(info.Out.Bytes*8, "", 2)+"/s", "\t")
		fmt.Fprint(tabs, info.In.Drops+info.Out.Drops, "\t")
		fmt.Fprint(tabs, info.In.Errors+info.Out.Errors, zstr.EscNoColor, "\n")
	}
	tabs.Flush()
}

func (defaultCommands) Command_delvar(c *CommandInfo, a struct {
	Path        string
	Description string `zui:"desc:Delete a variable. Use path to specify name or index of variable."`
}) {
	node, err := c.Session.PathAsItsTopNode(a.Path, VariableNode)
	if err != nil {
		c.Session.TermSession.Writeln(err)
		return
	}
	if node.Type != VariableNode {
		c.Session.TermSession.Writeln("Node is not a variable:", a.Path, reflect.TypeOf(node.Instance))
		return
	}
	do, _ := node.Instance.(Deleter)
	if do == nil {
		c.Session.TermSession.Writeln("Node is not deletable:", a.Path, reflect.TypeOf(node.Instance))
		return
	}
	uid := c.Session.TermSession.UserID()
	err = do.Delete(uid)
	if err != nil {
		c.Session.TermSession.Writeln("Error deleting variable:", err)
		return
	}
	c.Session.TermSession.Writeln("Deleted variable", "'"+node.Name+"'")
}
