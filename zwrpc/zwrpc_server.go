//go:build !js

package zwrpc

import (
	"net/http"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
	"nhooyr.io/websocket"
)

// https://golang.org/src/net/rpc/server.go?s=21953:21970#L708

var (
	NewClientHandler func(id string)
	// Calls                  = &WRPCCalls{}
)

var clients = map[string]*Client{}

func InitServer(certificatePath string, port int) {
	http.HandleFunc("/ws", acceptWS)
	go func() {
		znet.ServeHTTPInBackground(port, certificatePath, nil)
	}()
}

func acceptWS(w http.ResponseWriter, req *http.Request) {
	opts := websocket.AcceptOptions{}
	opts.InsecureSkipVerify = true // use OriginPatterns instead !!!

	vals := req.URL.Query()
	id := vals.Get("id")

	// zlog.Info("serveWS:", id, fromClientServer)

	wc, err := websocket.Accept(w, req, &opts)
	if err != nil {
		zlog.Error(err)
		wc.Close(websocket.StatusInternalError, err.Error())
		return
	}
	zlog.Info("Accept:", id, NewClientHandler != nil)
	c := &Client{}
	// wc.Token = id // id is not token?
	c.id = id
	c.ws = wc
	clients[id] = c

	c.pingRepeater = ztimer.RepeatIn(5, func() bool {
		zlog.Info("Ping repeat")
		// ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
		// defer cancel()
		// c.ws.Ping(ctx)
		// zlog.Info("Ping")
		return true
	})
	if NewClientHandler != nil {
		NewClientHandler(id)
	}

	repeatHandleIncoming(c, id)
}

/*
func repeatHandleAcceptFromClient(c *websocket.Conn, id string) error {
	wc := &Client{}
	// wc.Token = id // id is not token?
	wc.ws = c
	clients[id] = wc
	if NewClientHandler != nil {
		NewClientHandler(id)
	}
	zlog.Info("repeatHandleAcceptFromClient done", id)
	return nil
}
*/

func removeClient(client *Client) {
	for id, c := range clients {
		if c == client {
			c.pingRepeater.Stop()
			delete(clients, id)
			break
		}
	}
}

func repeatHandleIncoming(c *Client, id string) {
	for {
		cp, dbytes, err := readCallPayload(c.ws)
		if err != nil {
			cs := websocket.CloseStatus(err)
			switch cs {
			case websocket.StatusNormalClosure, websocket.StatusGoingAway, websocket.StatusAbnormalClosure: //, websocket.StatusServiceRestart:
				removeClient(c)
				return
			}
			//			c.Close(websocket.StatusInternalError, err.Error())
			zlog.Error(err, "readCallPlayload", dbytes)
			return
		}
		if cp.IDWildcard == "" {
			handleIncomingCall(c, cp)
		} else {
			zlog.Fatal(nil, "no forwarding")
			// forwardToClientServers(c, dbytes, cp.IDWildcard)
		}
	}
}

/*
func forwardToClientServers(c *Client, bytes []byte, wildcard string) {
	for id, client := range clients {
		if client == c {
			continue
		}
		matched, _ := filepath.Match(wildcard, id)
		if matched {
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
*/

func setNoVerifyClient(opts *websocket.DialOptions) {
	opts.HTTPClient = zhttp.NoVerifyClient
}

func CallAllClientsFromServer(method string, args interface{}, idWildcard string) error {
	var err error
	for id, client := range clients {
		if zstr.MatchWildcard(idWildcard, id) {
			zlog.Info("CallAll1:", id, idWildcard)
			e := client.Call(method, args, nil)
			zlog.Info("CallAll2:", id, idWildcard, e)
			if e != nil {
				err = e
			}
		}
	}
	return err
}
