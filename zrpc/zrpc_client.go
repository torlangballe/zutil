package zrpc

import (
	"encoding/json"
	"errors"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

// This is functionality to call rpc calls, using a client.

// Client is a type used to perform rpc calls
type Client struct {
	callURL                          string
	id                               string
	AuthToken                        string  // AuthToken is the token sent with the rpc call to authenticate or identify (for reverse calls)
	HandleAuthenticanFailedFunc      func()  // HandleAuthenticanFailedFunc is called if authentication fails
	TimeoutSecs                      float64 // A new client with a different timeout can be created. This is total time of comminication to server, execution, and returning the result
	KeepTokenOnAuthenticationInvalid bool    // if KeepTokenOnAuthenticationInvalid is true, the auth token isn't cleared on failure to authenticate
	SkipVerifyCertificate            bool    // if true, no certificate checking is done for https calls
}

// clientReceivePayload is structure to store received info from the call
type clientReceivePayload struct {
	Result                json.RawMessage
	Error                 string
	TransportError        TransportError
	AuthenticationInvalid bool
}

// MainClient is the main, default client. Is set in zapp, and used in zusers.
var MainClient *Client

// NewClient creates a client with a url prefix, adding zrest.AppURLPrefix
// This is
func NewClient(prefixURL string, id string) *Client {
	c := &Client{}
	c.callURL = zstr.Concat("/", prefixURL, zrest.AppURLPrefix, "zrpc")
	if id == "" {
		id = zstr.GenerateRandomHexBytes(10)
	}
	c.id = id
	c.TimeoutSecs = 100
	return c
}

// Call is used to execute a remote call. method is Type.MethodName
// inoput can be nil if not used, and result can be nil if not used/not in method.
func (c *Client) Call(method string, input, result any) error {
	var rp clientReceivePayload
	cp := CallPayload{Method: method, Args: input}
	cp.ClientID = c.id
	params := zhttp.MakeParameters()
	params.TimeoutSecs = c.TimeoutSecs
	params.SkipVerifyCertificate = c.SkipVerifyCertificate
	// params.PrintBody = true
	if c.AuthToken != "" {
		cp.Token = c.AuthToken
	}
	urlArgs := map[string]string{
		"method": method,
	}
	surl, _ := zhttp.MakeURLWithArgs(c.callURL, urlArgs)
	_, err := zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		return zlog.Error(err, "post")
	}
	if rp.AuthenticationInvalid { // check this first, will probably be an error also
		zlog.Info("zprc AuthenticationInvalid:", method, c.AuthToken)
		if !c.KeepTokenOnAuthenticationInvalid {
			c.AuthToken = ""
		}
		rp.TransportError = "authentication invalid"
		if c.HandleAuthenticanFailedFunc != nil {
			c.HandleAuthenticanFailedFunc()
		}
	}
	if rp.TransportError != "" {
		err = rp.TransportError
		return err
	}
	if rp.Error != "" {
		return errors.New(rp.Error)
	}
	if !rp.AuthenticationInvalid && result != nil {
		err = json.Unmarshal(rp.Result, result)
		if err != nil {
			zlog.Error(err, c.AuthToken, "unmarshal", string(rp.Result))
			return TransportError(err.Error())
		}
	}
	return nil
}

// CallWithTimeout is a convenience method that makes a copy of c with a new timeout
func (c *Client) CallWithTimeout(timeoutSecs float64, method string, args, result any) error {
	n := *c
	n.TimeoutSecs = timeoutSecs
	return n.Call(method, args, result)
}