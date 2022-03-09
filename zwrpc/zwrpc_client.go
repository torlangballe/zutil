package zwrpc

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Client struct {
	Token string
	ws    *websocket.Conn
}

var MainSendClient *Client

func NewClient(ipAddress string, port int, ipClientServerID string) (*Client, error) {
	var err error

	c := &Client{}
	c.Token = zstr.GenerateRandomHexBytes(8)
	if ipClientServerID != "" && !strings.Contains(ipClientServerID, ":") {
		ipClientServerID += ":" + c.Token
	}
	surl := fmt.Sprintf("wss://%s:%d/ws", ipAddress, port)
	if ipClientServerID != "" {
		surl += fmt.Sprintf("?id=%s", ipClientServerID)
	}
	ctx, _ := context.WithTimeout(context.Background(), time.Second*10)

	opts := websocket.DialOptions{}
	setNoVerifyClient(&opts)
	//	opts.HTTPClient = zhttp.NoVerifyClient
	c.ws, _, err = websocket.Dial(ctx, surl, &opts)
	if err != nil {
		zlog.Error(err)
		return nil, err
	}
	if ipClientServerID != "" {
		go repeatHandleClientCall(c.ws)
	}
	return c, nil
}

func repeatHandleClientCall(c *websocket.Conn) {
	for {
		cp, _, err := readCallPayload(c)
		if err != nil {
			//			c.Close(websocket.StatusInternalError, err.Error())
			zlog.Error(err, "readCallPlayload")
		}
		handleIncomingCall(c, cp)
	}
}

func NewSendAndReceiveClients(ipAddress, id string, port int) (send, receive *Client, err error) {
	send, err = NewClient(ipAddress, port, "")
	if err != nil {
		return
	}
	receive, err = NewClient(ipAddress, port, id)
	return
}

func (c *Client) Call(method string, args interface{}, result interface{}) error {
	return c.call(method, "", args, result)
}

func (c *Client) call(method, idWildcard string, args interface{}, result interface{}) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cp := callPayload{Method: method, Args: args, Token: c.Token, IDWildcard: idWildcard}

	err := wsjson.Write(ctx, c.ws, cp)
	if err != nil {
		return zlog.Error(err, "write")
	}

	var rp receivePayload
	rp.Result = result
	err = wsjson.Read(ctx, c.ws, &rp)
	if err != nil {
		zlog.Error(err, "read")
		return fmt.Errorf("%w: %v", TransportError, err)
	}
	zlog.Info("Call:", method, time.Since(start))
	if rp.Error != "" {
		return errors.New(rp.Error)
	}
	if rp.TransportError != "" {
		return fmt.Errorf("%w: %v", TransportError, rp.TransportError)
	}
	return nil
}

func (c *Client) CallClients(method string, args interface{}, results interface{}, idWildcard string) error {
	return c.call(method, "", args, results)
}

func (c *Client) Close() {
	c.ws.Close(websocket.StatusNormalClosure, "")
}
