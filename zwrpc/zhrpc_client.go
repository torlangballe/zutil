package zwrpc

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type HTTPClient struct {
	prefixURL string
	id        string
	token     string
}
type CallsBase int
type RPCCalls CallsBase

var MainHTTPClient *HTTPClient

func NewHTTPClient(prefixURL string, id string) *HTTPClient {
	c := &HTTPClient{}
	c.prefixURL = prefixURL
	if id == "" {
		id = zstr.GenerateRandomHexBytes(10)
	}
	c.id = id
	zlog.Info("NewHTTPClient:", prefixURL, id)
	return c
}

func (c *HTTPClient) Call(method string, args, result any) error {
	start := time.Now()
	var rp receivePayload
	cp := callPayload{Method: method, Args: args}
	params := zhttp.MakeParameters()
	if c.token != "" {
		params.Headers["X-Token"] = c.token
	}
	urlArgs := map[string]string{
		"id":     c.id,
		"method": method,
	}
	surl, _ := zhttp.MakeURLWithArgs(c.prefixURL+"/xrpc", urlArgs)
	_, err := zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		return err
	}
	if rp.TransportError != "" {
		return fmt.Errorf("%w: %v", TransportError, rp.TransportError)
	}
	zlog.Info("Call:", method, time.Since(start), result, rp.Error)
	return nil
}
