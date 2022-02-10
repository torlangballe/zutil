//go:build !js

package zwsrpc

/*
import (
	"net/http"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"nhooyr.io/websocket"
)

// https://golang.org/src/net/rpc/server.go?s=21953:21970#L708

var (
	NewClientServerHandler func(id string)
)

var ClientRPCServers = map[string]Client{}

func InitServer(certFilesSuffix string, port int) {
	http.HandleFunc("/ws", acceptWS)
	zlog.Info("zwsrpc.InitServer", certFilesSuffix, port)
	go func() {
		znet.ServeHTTPInBackground(port, certFilesSuffix, nil)
	}()
}

func handleAcceptFromClientServer(c *websocket.Conn, id string) error {
	var wc Client
	wc.Token = id
	wc.ws = c
	ClientRPCServers[id] = wc
	if NewClientServerHandler != nil {
		NewClientServerHandler(id)
	}
	// zlog.Info("handleConnectAsRPCServer done")
	return nil
}

func acceptWS(w http.ResponseWriter, req *http.Request) {
	opts := websocket.AcceptOptions{}
	opts.InsecureSkipVerify = true // use OriginPatterns instead !!!

	vals := req.URL.Query()
	id := vals.Get("id")

	fromClientServer := (vals.Get("cs") != "")
	// zlog.Info("serveWS:", id, fromClientServer)

	c, err := websocket.Accept(w, req, &opts)
	if err != nil {
		zlog.Error(err)
		c.Close(websocket.StatusInternalError, err.Error())
		return
	}
	if fromClientServer {
		go handleAcceptFromClientServer(c, id)
	} else {
		go handleIncoming(c)
	}
}

func setNoVerifyClient(opts *websocket.DialOptions) {
	opts.HTTPClient = zhttp.NoVerifyClient
}

*/
