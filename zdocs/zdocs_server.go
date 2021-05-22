// +build server

package zdocs

import (
	"net/http"

	"github.com/torlangballe/zutil/zrpc"
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

type DocCalls zrpc.CallsBase

var Calls = new(DocCalls)

func (c *DocCalls) GetDocument(req *http.Request, get *GetDoc, got *GetDocGot) error {
	return nil
}

func (c *DocCalls) PutDocument(req *http.Request, put *PutDoc, result *zrpc.Any) error {
	return nil
}

func (c *DocCalls) Flush(req *http.Request, info *DocBase, recept *string) error {
	return nil
}
