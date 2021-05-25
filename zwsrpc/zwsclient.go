package zwsrpc

import (
	"context"
	"errors"
	"fmt"
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

func NewClient(ipAddress, id string, port int, iAmRPCServer bool) (*Client, error) {
	var err error

	c := &Client{}
	if id == "" {
		id = zstr.GenerateRandomHexBytes(8)
	}
	c.Token = id
	surl := fmt.Sprintf("wss://%s:%d/ws?id=%s", ipAddress, port, id)
	if iAmRPCServer {
		surl += ("&cs=1")
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
	if iAmRPCServer {
		go handleIncoming(c.ws)
	}
	return c, nil
}

func NewClients(ipAddress, id string, port int, send, receive **Client) (err error) {
	iAmRPCServer := true
	*send, err = NewClient(ipAddress, id, port, !iAmRPCServer)
	if err != nil {
		return
	}
	*receive, err = NewClient(ipAddress, id, port, iAmRPCServer)
	return err
}

func (c *Client) CallRPC(name string, arg interface{}, result interface{}) error {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	cp := callPayload{Name: name, Arg: arg, Token: c.Token}

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
	zlog.Info("CallRPC:", name, time.Since(start))
	if rp.Error != "" {
		return errors.New(rp.Error)
	}
	if rp.TransportError != "" {
		return fmt.Errorf("%w: %v", TransportError, rp.TransportError)
	}
	return nil
}
