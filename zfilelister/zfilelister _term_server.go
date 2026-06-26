//go:build server

package zfilelister

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/blacktop/go-termimg"
	"github.com/torlangballe/zutil/zcommands"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
)

type FilesCom struct {
	Server   *FileServer `zui:"-"`
	FullPath string      `zui:"-"`
}

type FileCom struct {
	Server   *FileServer `zui:"-"`
	FullPath string      `zui:"-"`
}

func (fs *FileServer) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	zlog.Info("FileServer CommandNodes:", wild, forExpand)
	var nodes []zcommands.Node
	for _, name := range zmap.SortedStringKeys(fs.folders) {
		fc := FilesCom{
			Server:   fs,
			FullPath: fs.folders[name],
		}
		n := zcommands.MakeNode(name, zcommands.ComNode, fc, 0)
		nodes = append(nodes, n)
	}
	return nodes
}

func (fc FilesCom) CommandNodes(s *zcommands.Session, wild string, forExpand bool) []zcommands.Node {
	zlog.Info("FilesCom CommandNodes:", fc.FullPath)
	var paths []string
	walkOpts := zfile.WalkOptionGiveFolders
	err := zfile.Walk(fc.FullPath, wild, walkOpts, func(fpath string, info os.FileInfo) error {
		if info.IsDir() {
			fpath += "/"
		}
		paths = append(paths, fpath)
		return nil
	})
	if err != nil {
		s.TermSession.Writeln("Error walking path:", fc.FullPath, err)
		return nil
	}
	var nodes []zcommands.Node
	for _, path := range paths {
		_, name := filepath.Split(strings.TrimRight(path, "/"))
		zlog.Info("PATH:", path, name)
		isDir := strings.HasSuffix(path, "/")
		n := zcommands.MakeNode(name, zcommands.RowNode|zcommands.ComNode, name, 0)
		if isDir {
			var fc2 FilesCom
			// fc2.Name = name
			fc2.Server = fc.Server
			fc2.FullPath = path
			n.Instance = fc2
		} else {
			var fc2 FileCom
			// fc2.Name = name
			fc2.FullPath = path
			fc2.Server = fc.Server
			n.Instance = fc2
		}
		nodes = append(nodes, n)
	}
	return nodes
}

func (fc FileCom) DumpToTerminal(s *zcommands.Session) {
	img, err := termimg.Open(fc.FullPath)
	img = img.Width(100).Height(100).Scale(termimg.ScaleFit)
	if err != nil {
		s.TermSession.Writeln(err)
		return
	}
	renderedString, err := img.Render()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to render image: %v\n", err)
		os.Exit(1)
	}
	s.TermSession.Write(renderedString)
	s.TermSession.Writeln("")
}
