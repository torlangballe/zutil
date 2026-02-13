package xrpc

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwebsocket"
)

type ConnectInfo[C any] struct {
	connection         *C
	currentBackoffSecs float64
	maxBackoffSecs     float64
	lastConnectTry     time.Time
	// connectFunc        func(pipeID string) *C
}

type RPC struct {
	clients           map[string]*ConnectInfo[zwebsocket.Client]
	servers           map[string]*ConnectInfo[zwebsocket.Server]
	Executor          *znamedfuncs.Executor
	ConnectServerFunc func(serverID string) (*zwebsocket.Server, error)
	ConnectClientFunc func(clientID string) (*zwebsocket.Client, error)
	connectRepeater   *ztimer.Repeater
}

func NewRPC() *RPC {
	r := &RPC{}
	r.clients = make(map[string]*ConnectInfo[zwebsocket.Client])
	r.servers = make(map[string]*ConnectInfo[zwebsocket.Server])
	r.connectRepeater = ztimer.NewRepeater()
	return r
}

func (ci *ConnectInfo[C]) ConnectIfNeeded(id string, connectFunc func(id string) (*C, error)) error {
	if ci.connection != nil {
		return nil
	}
	if time.Since(ci.lastConnectTry).Seconds() < ci.currentBackoffSecs {
		return nil
	}
	connection, err := connectFunc(id)
	// zlog.Warn("ConnectIfNeeded", id, connection != nil, zlog.Pointer(ci), err)
	if err != nil {
		return err
	}
	if connection == nil {
		if ci.currentBackoffSecs == 0 {
			ci.currentBackoffSecs = 0.1
		} else {
			ci.currentBackoffSecs *= 2
		}
		return nil
	}
	ci.connection = connection
	ci.currentBackoffSecs = 0
	return nil
}

func (r *RPC) ClientForID(clientID string) *zwebsocket.Client {
	c := r.clients[clientID]
	if c != nil {
		return c.connection
	}
	return nil
}

func (r *RPC) ServerForID(serverID string) *zwebsocket.Server {
	s := r.servers[serverID]
	if s != nil {
		return s.connection
	}
	return nil
}

func (r *RPC) AddClient(clientID string) {
	c := ConnectInfo[zwebsocket.Client]{
		maxBackoffSecs: 5,
	}
	r.clients[clientID] = &c
	c.ConnectIfNeeded(clientID, r.ConnectClientFunc)
}

func (r *RPC) AddServer(serverID string) {
	s := ConnectInfo[zwebsocket.Server]{
		maxBackoffSecs: 5,
	}
	r.servers[serverID] = &s
	s.ConnectIfNeeded(serverID, r.ConnectServerFunc)
}

func (r *RPC) Start() {
	r.connectRepeater.Set(0.1, true, func() bool {
		for id, c := range r.clients {
			err := c.ConnectIfNeeded(id, r.ConnectClientFunc)
			if err != nil {
				r.handleClientError(id, err)
			}
		}
		for id, s := range r.servers {
			s.ConnectIfNeeded(id, r.ConnectServerFunc)
		}
		return true
	})
}

func (r *RPC) Close() {
	r.connectRepeater.Stop()
}

func (r *RPC) handleServerConnectionError(pipeID string, err error) {

}

func (r *RPC) handleClientError(pipeID string, err error) {
	c := r.clients[pipeID]
	// zlog.Warn("handleClientError:", pipeID, c != nil, err)
	if c != nil && c.connection != nil {
		// zlog.Warn("handleClientError in:", pipeID)
		c.connection.Close()
		c.connection = nil
	}
}

func (r *RPC) MakeServer(path string, port int) (*zwebsocket.Server, error) {
	handler := func(id string, msg []byte, err error) []byte {
		if err != nil {
			// zlog.Warn("RPC server got error from websocket connection", id, err)
			return nil
		}
		ci := znamedfuncs.CallerInfo{
			CallerID: id,
		}
		var result []byte
		err = r.Executor.ExecuteFromToJSON(msg, &result, ci)
		zlog.OnError(err, "RPC server call execute error", msg)
		return result
	}
	return zwebsocket.NewServer(path, port, handler)
}

func (r *RPC) MakeClient(url, pipeID string) (*zwebsocket.Client, error) {
	var client *zwebsocket.Client
	handler := func(msg []byte, err error) []byte {
		if err != nil {
			r.handleClientError(pipeID, err)
			return nil
		}
		ci := znamedfuncs.CallerInfo{
			CallerID: pipeID,
		}
		ci.TimeToLiveSeconds = client.DefaultTimeToLiveSeconds
		var result []byte
		err = r.Executor.ExecuteFromToJSON(msg, &result, ci)
		zlog.OnError(err, pipeID)
		// zlog.OnError(err, "RPC client call execute error", msg)
		return result
	}
	var err error
	client, err = zwebsocket.NewClient(pipeID, url, handler)
	if err != nil {
		r.handleClientError(pipeID, err)
		return nil, err
	}
	return client, nil
}

func (r *RPC) RemoveClient(pipeID string) {
	c := r.clients[pipeID]
	if c.connection != nil {
		c.connection.Close()
	}
	delete(r.clients, pipeID)
}

func (r *RPC) Call(pipeID string, fullMethod string, in any, resultPtr any, cis ...znamedfuncs.CallerInfo) error {
	var cp znamedfuncs.CallPayloadSend
	cp.Method = fullMethod
	if len(cis) > 0 {
		cp.CallerInfo = cis[0]
	}
	var err error
	cp.CallerInfo.CallerID = pipeID
	cp.Args = in
	if err != nil {
		return err
	}
	cpJson, err := json.Marshal(cp)
	if err != nil {
		return err
	}
	if pipeID == "" {
		if len(r.clients) != 1 {
			return fmt.Errorf(`pipeID=="" but 0/multiple clients`)
		}
		pipeID = zmap.GetAnyKeyAsString(r.clients)
	}
	var rpJson []byte
	c := r.clients[pipeID]
	if c != nil {
		if c.connection == nil {
			return zlog.NewError("Connection down for pipe:", pipeID, "method:", fullMethod)
		}
		rpJson, err = c.connection.Exchange(cpJson)
		if err != nil {
			r.handleClientError(pipeID, err)
			return err
		}
	} else {
		var server *zwebsocket.Server
		// zlog.Warn("RPC Server Call to pipeID:", pipeID, len(r.servers))
		if len(r.servers) == 0 {
			return zlog.NewError("RPC Call with no server and no client for pipeID:", pipeID)
		}
		if pipeID == "" {
			if len(r.servers) == 1 {
				for _, s := range r.servers {
					if len(s.connection.Connections) == 1 {
						pipeID = s.connection.Connections[0].ID
						server = s.connection
						break
					}
				}
			}
		} else {
			for _, s := range r.servers {
				if s.connection == nil {
					continue
				}
				for _, c := range s.connection.Connections {
					if c.ID == pipeID {
						server = s.connection
						break
					}
				}
			}
		}
		if server == nil {
			// for _, s := range r.servers {
			// 	zlog.Warn("ServC:", s.connection != nil, zlog.Pointer(s))
			// }
			return zlog.NewError("RPC Call with no id and not just one client or connection", pipeID)
		}
		rpJson, err = server.ExchangeWithID(pipeID, cpJson)
		// zlog.Warn("RPC ServerConnection Call to pipeID:", pipeID, err)
	}
	if err != nil {
		r.handleServerConnectionError(pipeID, err)
		return err
	}
	var rp znamedfuncs.ReceivePayload
	err = json.Unmarshal(rpJson, &rp)
	// zlog.Warn("RPC Call to pipeID:", pipeID, "method:", fullMethod, "args:", in, "got result json:", string(rpJson), "err:", err)
	if err != nil {
		return zlog.NewError(err, "unmarshal RP failed json:"+string(rpJson))
	}
	if resultPtr != nil {
		err = json.Unmarshal(rp.Result, resultPtr)
		if err != nil {
			return zlog.NewError(err, "unmarshal RP.Result payload failed")
		}
	}
	if rp.Error != "" {
		return fmt.Errorf("RPC call result error: %s", rp.Error)
	}
	if rp.TransportError != "" {
		return fmt.Errorf("RPC call transport error: %s", rp.TransportError)
	}
	return nil
}
