package zwrpc

import (
	"encoding/json"
	"fmt"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type HTTPClient struct {
	prefixURL string
	id        string
	token     string
}

var MainHTTPClient *HTTPClient

func NewHTTPClient(prefixURL string, id string) *HTTPClient {
	c := &HTTPClient{}
	c.prefixURL = prefixURL
	if id == "" {
		id = zstr.GenerateRandomHexBytes(10)
	}
	c.id = id
	// zlog.Info("NewHTTPClient:", prefixURL, id)
	return c
}

type clientReceivePayload struct {
	Result         json.RawMessage
	Error          string
	TransportError string
}

func (c *HTTPClient) Call(method string, args, result any) error {
	// start := time.Now()
	var rp clientReceivePayload
	cp := callPayload{Method: method, Args: args}
	cp.ClientID = c.id
	params := zhttp.MakeParameters()
	// params.PrintBody = true
	if c.token != "" {
		params.Headers["X-Token"] = c.token
	}
	urlArgs := map[string]string{
		"method": method,
	}
	surl, _ := zhttp.MakeURLWithArgs(c.prefixURL+"/xrpc", urlArgs)
	_, err := zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		zlog.Error(err, "post")
		return err
	}
	if rp.TransportError != "" {
		return fmt.Errorf("%w: %v", TransportError, rp.TransportError)
	}
	if result != nil {
		// zlog.Info("UNMARSH:", string(rp.Result))
		err = json.Unmarshal(rp.Result, result)
		if err != nil {
			zlog.Error(err, "unmarshal")
			return err
		}
	}
	// zlog.Info("Called:", method, time.Since(start), result)
	return nil
}
