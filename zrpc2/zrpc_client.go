package zrpc2

import (
	"encoding/json"
	"errors"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

type Client struct {
	prefixURL                   string
	id                          string
	UseAuth                     bool
	AuthToken                   string
	HandleAuthenticanFailedFunc func()
	TimeoutSecs                 float64
}

type clientReceivePayload struct {
	Result                json.RawMessage
	Error                 string
	TransportError        string
	AuthenticationInvalid bool
}

var MainClient *Client

func NewClient(prefixURL string, id string) *Client {
	c := &Client{}
	c.prefixURL = zstr.Concat("/", prefixURL, zrest.AppURLPrefix)
	// zlog.Info("NewClient:", c.prefixURL, htype)
	if id == "" {
		id = zstr.GenerateRandomHexBytes(10)
	}
	c.id = id
	c.TimeoutSecs = 20
	// zlog.Info("NewClient:", prefixURL, id)
	return c
}

func (c *Client) Call(method string, args, result any) error {
	// start := time.Now()
	var rp clientReceivePayload
	cp := callPayload{Method: method, Args: args}
	cp.ClientID = c.id
	params := zhttp.MakeParameters()
	params.TimeoutSecs = c.TimeoutSecs
	// params.PrintBody = true
	if c.AuthToken != "" {
		params.Headers["X-Token"] = c.AuthToken
	}
	urlArgs := map[string]string{
		"method": method,
	}
	spath := zstr.Concat("/", c.prefixURL, "xrpc")
	surl, _ := zhttp.MakeURLWithArgs(spath, urlArgs)
	// zlog.Info("Call:", surl, c.prefixURL)
	_, err := zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		// zlog.Error(err, "post")
		return err
	}
	// zlog.Info("Called:", zlog.Full(rp))
	if rp.AuthenticationInvalid { // check this first, will probably be an error also
		c.AuthToken = ""
		rp.TransportError = "authentication invalid"
		if c.HandleAuthenticanFailedFunc != nil {
			c.HandleAuthenticanFailedFunc()
		}
	}
	if rp.Error != "" {
		return errors.New(rp.Error)
	}
	if rp.TransportError != "" {
		err = &TransportError{Text: rp.TransportError}
		return err
	}
	if !rp.AuthenticationInvalid && result != nil {
		err = json.Unmarshal(rp.Result, result)
		if err != nil {
			zlog.Error(err, c.AuthToken, "unmarshal", string(rp.Result))
			return &TransportError{Text: err.Error()}
		}
	}
	// zlog.Info("Called:", method, time.Since(start), result)
	return nil
}
