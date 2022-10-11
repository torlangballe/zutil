//go:build server
// +build server

package zdocs

import (
	"github.com/torlangballe/zutil/zrpc2"
)

type DocBase struct {
	UserID    string
	Password  string
	Container string // could be repository url
	Path      string
}

type GetDoc struct {
	DocBase
}

type GetDocGot struct {
	Text string
}

type PutDoc struct {
	DocBase
	Text string
}

type DocCalls zrpc2.CallsBase

var Calls = new(DocCalls)

func (c *DocCalls) GetDocument(get *GetDoc, got *GetDocGot) error {
	return nil
}

func (c *DocCalls) PutDocument(put *PutDoc, result *zrpc2.Unused) error {
	return nil
}

func (c *DocCalls) Flush(info *DocBase, recept *string) error {
	return nil
}
