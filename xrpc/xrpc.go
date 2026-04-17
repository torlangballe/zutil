package xrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwebsocket"
)

type ConnectInfo[C any] struct {
	connection         *C
	currentBackoffSecs float64
	maxBackoffSecs     float64
	lastConnectTry     time.Time
	targetID           int64
}

type RPC struct {
	Executor                         *znamedfuncs.Executor
	ConnectServerFunc                func(serverID string) (*zwebsocket.Server, error)
	ConnectClientFunc                func(clientID string) (*zwebsocket.Client, error)
	HandleAuthenticationFailedFunc   func(id string) // HandleAuthenticationFailedFunc is called if authentication fails
	KeepTokenOnAuthenticationInvalid bool            // if KeepTokenOnAuthenticationInvalid is true, the auth token isn't cleared on failure to authenticate
	IPAddress                        string          // IP address to report in ClientInfo for outgoing calls.
	targetID                         int64
	waitForStart                     *zprocess.OnceWait

	clients         map[string]*ConnectInfo[zwebsocket.Client]
	servers         map[string]*ConnectInfo[zwebsocket.Server]
	connectRepeater *ztimer.Repeater
}

type Caller struct {
	RPC *RPC
	ID  string
}

const (
	MainClientID = "mainclient"
	MainServerID = "mainserver"
)

var (
	MainRPC                *RPC
	exchangeWithServerFunc func(r *RPC, pipeID string, cpJson []byte) (rpJson []byte, err error)
	xRPCLog                = zlog.NewEnabler()
)

func NewRPC() *RPC {
	r := &RPC{}
	r.clients = make(map[string]*ConnectInfo[zwebsocket.Client])
	r.servers = make(map[string]*ConnectInfo[zwebsocket.Server])
	r.connectRepeater = ztimer.NewRepeater()
	r.targetID = rand.Int63()
	r.waitForStart = zprocess.NewOnceWait()
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

func (r *RPC) SetClient(clientID string) {
	zlog.Info("SetClient called for clientID:", clientID)
	c, has := r.clients[clientID]
	if !has {
		c = &ConnectInfo[zwebsocket.Client]{
			maxBackoffSecs: 5,
		}
		r.clients[clientID] = c
	}
	c.ConnectIfNeeded(clientID, r.ConnectClientFunc)
}

func (r *RPC) SetServer(serverID string) {
	s, has := r.servers[serverID]
	if !has {
		s = &ConnectInfo[zwebsocket.Server]{
			maxBackoffSecs: 5,
		}
		r.servers[serverID] = s
	}
	s.ConnectIfNeeded(serverID, r.ConnectServerFunc)
}

func (r *RPC) Start() {
	r.waitForStart.Done() // allow incoming calls to be handled now that we're starting
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

// MakeClient creates a new client connection to the given URL. If port is not 0, it overrides the port in the URL with the given port.
// Note that you clients are normally made with AddClient() that uses ConnectIfNeeded() and r.ConnectClientFunc to actually create a client.
func (r *RPC) MakeClient(surl, pipeID string, port int) (*zwebsocket.Client, error) {
	zlog.Info("RPC.MakeClient:", surl, pipeID, port)
	var client *zwebsocket.Client
	handler := func(msg []byte, err error) []byte {
		if err != nil {
			r.handleClientError(pipeID, err)
			return nil
		}
		r.waitForStart.Wait() // wait for Start() to be called before handling any messages
		ci := zrpc.ClientInfo{
			Token:    client.AuthToken,
			ClientID: pipeID,
		}
		ci.TimeToLiveSeconds = client.DefaultTimeToLiveSeconds
		var result []byte
		err = r.Executor.ExecuteFromToJSON(msg, &result, ci, r.targetID)
		zlog.OnError(err, pipeID)
		if err == znamedfuncs.AuthenticationInvalidError {
			if !r.KeepTokenOnAuthenticationInvalid {
				client.AuthToken = ""
			}
			if r.HandleAuthenticationFailedFunc != nil {
				r.HandleAuthenticationFailedFunc(pipeID)
			}
		}
		// zlog.OnError(err, "RPC client call execute error", msg)
		return result
	}
	var err error
	if port != 0 {
		url, err := url.Parse(surl)
		if err != nil {
			zlog.OnError(err, "Parse URL failed:", surl)
			return nil, err
		}
		url.Host = fmt.Sprintf("%s:%d", url.Hostname(), port)
		surl = url.String()
	}
	client, err = zwebsocket.NewClient(pipeID, surl, handler)
	if err != nil {
		r.handleClientError(pipeID, err)
		return nil, err
	}
	return client, nil
}

func (r *RPC) RemoveClient(pipeID string) {
	c := r.clients[pipeID]
	if c == nil {
		return
	}
	if c.connection != nil {
		c.connection.Close()
	}
	delete(r.clients, pipeID)
}

func (r *RPC) MakeCaller(pipeID string) Caller {
	return Caller{
		RPC: r,
		ID:  pipeID,
	}
}

func MainCaller() Caller {
	return MainRPC.MakeCaller(MainClientID)
}

func MainClient() *zwebsocket.Client {
	if MainRPC == nil {
		return nil
	}
	c := MainRPC.ClientForID(MainClientID)
	// zlog.Info("MainClient:", c != nil, MainRPC != nil, MainClientID)
	// zlog.Info("MainClient:", MainRPC.clients)
	return c
}

func (c Caller) Call(fullMethod string, in any, resultPtr any) error {
	return c.RPC.Call(c.ID, fullMethod, in, resultPtr)
}

func (r *RPC) TokenForClientID(clientID string) (string, error) {
	c := r.clients[clientID]
	if c != nil && c.connection != nil {
		return c.connection.AuthToken, nil
	}
	return "", errors.New("not found")
}

func (r *RPC) Call(pipeID string, fullMethod string, in any, resultPtr any, timeoutSecs ...float64) error {
	var cp znamedfuncs.CallPayloadSend
	cp.Method = fullMethod
	c := r.clients[pipeID]
	var err error
	cp.ClientInfo.ClientID = pipeID
	if c != nil {
		cp.ClientInfo.Token = c.connection.AuthToken
		cp.ClientInfo.TimeToLiveSeconds = c.connection.DefaultTimeToLiveSeconds
		cp.ClientInfo.SendDate = time.Now().UTC()
		cp.ClientInfo.IPAddress = r.IPAddress
		cp.TargetID = c.targetID
	}
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
	if c != nil {
		if c.connection == nil {
			return zlog.NewError("Connection down for client pipe:", pipeID, "method:", fullMethod)
		}
		if len(timeoutSecs) > 0 {
			c.connection.DefaultTimeToLiveSeconds = timeoutSecs[0]
		}
		rpJson, err = c.connection.Exchange(cpJson)
		if err != nil {
			r.handleClientError(pipeID, err)
			return err
		}
	} else if exchangeWithServerFunc != nil {
		rpJson, err = exchangeWithServerFunc(r, pipeID, cpJson)
	}
	if err != nil {
		r.handleServerConnectionError(pipeID, err)
		return err
	}
	var rp znamedfuncs.ReceivePayload
	err = json.Unmarshal(rpJson, &rp)
	zlog.Info(xRPCLog, "RPC Call to pipeID:", pipeID, "method:", fullMethod, "args:", in, "got result json:", string(rpJson), "err:", err)
	if err != nil {
		return zlog.NewError(err, "unmarshal RP failed json:"+string(rpJson))
	}
	c = r.clients[pipeID] // let's get it again in case it was removed
	if c != nil {
		c.targetID = rp.ExecutorTargetID // update client TargetID to match the executor that executed the call, in case it changed after a restart
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
