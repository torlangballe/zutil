//go:build !js

package zwrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"nhooyr.io/websocket"
)

// https://golang.org/src/net/rpc/server.go?s=21953:21970#L708

// type WRPCCalls struct{}

var (
	NewClientServerHandler func(id string)
	// Calls                  = &WRPCCalls{}
)

// func (c *WRPCCalls) CallClients(method string, wildcard string, args interface{}) {
// 	for id, c := range clientRPCServers {

// 	}
// }

var clientRPCServers = map[string]Client{}

func InitServer(certificatePath string, port int) {
	http.HandleFunc("/ws", acceptWS)
	go func() {
		znet.ServeHTTPInBackground(port, certificatePath, nil)
	}()
}

func repeatHandleAcceptFromClientServer(c *websocket.Conn, id string) error {
	var wc Client
	// wc.Token = id // id is not token?
	wc.ws = c
	clientRPCServers[id] = wc
	if NewClientServerHandler != nil {
		NewClientServerHandler(id)
	}
	zlog.Info("repeatHandleAcceptFromClientServer done")
	return nil
}

func acceptWS(w http.ResponseWriter, req *http.Request) {
	opts := websocket.AcceptOptions{}
	opts.InsecureSkipVerify = true // use OriginPatterns instead !!!

	vals := req.URL.Query()
	id := vals.Get("id")

	fromClientServer := (id != "")
	// zlog.Info("serveWS:", id, fromClientServer)

	wc, err := websocket.Accept(w, req, &opts)
	if err != nil {
		zlog.Error(err)
		wc.Close(websocket.StatusInternalError, err.Error())
		return
	}
	if fromClientServer {
		go repeatHandleAcceptFromClientServer(wc, id)
	} else {
		if err != nil {

		}
		go repeatHandleIncoming(wc)
	}
}

func repeatHandleIncoming(c *websocket.Conn) {
	for {
		cp, dbytes, err := readCallPayload(c)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return
			}
			//			c.Close(websocket.StatusInternalError, err.Error())
			zlog.Error(err, "readCallPlayload")
			return
		}
		if cp.IDWildcard == "" {
			handleIncomingCall(c, cp)
		} else {
			forwardToClientServers(c, dbytes, cp.IDWildcard)
		}
	}
}

func forwardToClientServers(c *websocket.Conn, bytes []byte, wildcard string) {
	for id, client := range clientRPCServers {
		matched, _ := filepath.Match(wildcard, id)
		if matched {
			ctx := context.Background()
			var rp receivePayload
			writer, err := client.ws.Writer(ctx, websocket.MessageText)
			if err != nil {
				zlog.Error(err, "Error getting writer:", id)
				continue
			}
			n, err := writer.Write(bytes)
			if err != nil {
				zlog.Error(err, "Error copying to client server:", id, n)
				continue
			}
			_, reader, err := c.Reader(ctx)
			if err != nil {
				zlog.Error(err, "get reader:", wildcard)
				continue
			}
			d := json.NewDecoder(reader)
			err = d.Decode(&rp)
			if err != nil {
				zlog.Error(err, "decode")
				continue
			}
			if rp.Error != "" {
				zlog.Error(errors.New(rp.Error), "result error")
			}
			if rp.TransportError != "" {
				zlog.Error(errors.New(rp.TransportError), "result error")
			}
		}
	}
}

func setNoVerifyClient(opts *websocket.DialOptions) {
	opts.HTTPClient = zhttp.NoVerifyClient
}
