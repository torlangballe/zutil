package zwrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
	"nhooyr.io/websocket"
)

type Client struct {
	// Token string
	id           string
	ws           *websocket.Conn
	ipAddress    string
	port         int
	ssl          bool
	keepOpenSecs float64
	pingRepeater *ztimer.Repeater
}

var MainSendClient *Client

func NewClient(ipAddress string, port int, ssl bool, id string, keepOpenSecs float64) (*Client, error) {
	c := &Client{}
	c.ipAddress = ipAddress
	c.port = port
	c.ssl = ssl
	c.id = id
	c.keepOpenSecs = keepOpenSecs
	err := c.connect()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connect() error {
	var err error
	pref := "ws"
	if c.ssl {
		pref = "wss"
	}
	surl := fmt.Sprintf("%s://%s:%d/ws", pref, c.ipAddress, c.port)
	if c.id != "" {
		surl += fmt.Sprintf("?id=%s", c.id)
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	opts := websocket.DialOptions{}
	setNoVerifyClient(&opts)
	//	opts.HTTPClient = zhttp.NoVerifyClient
	c.ws, _, err = websocket.Dial(ctx, surl, &opts)
	if err != nil {
		zlog.Error(err)
		return err
	}
	if c.id != "" {
		go c.repeatHandleClientCall()
	}
	return nil
}

func (c *Client) repeatHandleClientCall() {
	for {
		cp, _, err := readCallPayload(c.ws)
		// zlog.Info("repeatHandleClientCall", err, c.id)
		cs := websocket.CloseStatus(err)
		switch cs {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway, websocket.StatusAbnormalClosure, websocket.StatusServiceRestart:
			zlog.Info("repeatHandleClientCall close", c.id)
			c.ws.Close(cs, "closing on received close type error")
			if c.keepOpenSecs == 0 {
				c.ws = nil
				return
			}
			for {
				err = c.connect()
				if err == nil {
					break
				}
				time.Sleep(ztime.SecondsDur(c.keepOpenSecs))
			}
			return
		}
		if err != nil {
			//			c.Close(websocket.StatusInternalError, err.Error())
			zlog.Error(err, "readCallPlayload")
		}
		handleIncomingCall(c, cp)
	}
}

/*
func NewSendAndReceiveClients(ipAddress, id string, ssl bool, port int) (send, receive *Client, err error) {
	send, err = NewClient(ipAddress, port, ssl, "")
	if err != nil {
		return
	}
	if id == "" {
		id = zstr.GenerateRandomHexBytes(10)
	}
	receive, err = NewClient(ipAddress, port, ssl, id)
	return
}
*/

func (c *Client) Call(method string, args interface{}, result interface{}) error {
	return c.call(method, "", args, result)
}

func (c *Client) call(method, idWildcard string, args interface{}, result interface{}) error {
	start := time.Now()
	cp := callPayload{Method: method, Args: args, IDWildcard: idWildcard, ClientID: c.id}
	data, err := json.Marshal(cp)
	if err != nil {
		return zlog.Error(err, "marshal")
	}
	resultData, err := SendReceiveDataToWS(c.id, c.ws, data)
	if err != nil {
		return zlog.Error(err, "send-receive")
	}
	var rp receivePayload
	rp.Result = result
	err = json.Unmarshal(resultData, result)
	if err != nil {
		zlog.Error(err, "unmarshal")
		return fmt.Errorf("%w: %v", TransportError, err)
	}
	if rp.Error != "" {
		return errors.New(rp.Error)
	}
	if rp.TransportError != "" {
		return fmt.Errorf("%w: %v", TransportError, rp.TransportError)
	}
	zlog.Info("Call:", method, time.Since(start), result, rp.Error)
	return nil
}

func (c *Client) Close() {
	c.ws.Close(websocket.StatusNormalClosure, "")
}

func (c *Client) CallClientsViaServer(method string, args interface{}, results interface{}, idWildcard string) error {
	return c.call(method, idWildcard, args, results)

}
