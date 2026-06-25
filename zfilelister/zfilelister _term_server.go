//go:build server

package zfilelister

import (
	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
)

type FilesCom struct {
	Server  *FileServer `zui:"-"`
	DirOpts DirOptions  `zui:"-"`
	Name    string      `zui:"-"`
	Path    string      `zui:"-"`
}

func (fs *FileServer) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	var nodes []zcommands.Node
	for _, name := range zmap.SortedStringKeys(fs.folders) {
		baseFolder := fs.folders[name]
		fc := FilesCom{
			DirOpts: DirOptions{
				StoreName: name,
				PathStub:  baseFolder,
			},
			Name:   name,
			Server: fs,
		}
		n := zcommands.MakeNode(name, zcommands.ComNode, fc, 0)
		nodes = append(nodes, n)
	}
	return nodes
}

func (fc FilesCom) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	var paths []string
	err := fc.Server.getDirectory(fc.DirOpts, &paths)
	if err != nil {
		s.TermSession.Writeln(err)
		return nil
	}
	var nodes []zcommands.Node
	for _, path := range paths {
		isDir := zstr.HasSuffix(path, "/", &path)
		n := zcommands.MakeNode(path, zcommands.RowNode, path, 0)
		if isDir {
			n.Type |= zcommands.ComNode
			var fc2 FilesCom
			fc2.DirOpts = fc.DirOpts
			fc2.DirOpts.PathStub = zfile.JoinPathParts(fc.DirOpts.PathStub, path)
			fc2.Name = path
			fc2.Server = fc.Server
			n.Instance = fc2
		}
		nodes = append(nodes, n)
	}
	return nodes
}
