package zrpc

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// This is functionality to call rpc calls, using a client.

// Client is a type used to perform rpc calls
type Client struct {
	AuthToken                        string  // AuthToken is the token sent with the rpc call to authenticate or identify (for reverse calls)
	HandleAuthenticanFailedFunc      func()  // HandleAuthenticanFailedFunc is called if authentication fails
	TimeoutSecs                      float64 // A new client with a different timeout can be created. This is total time of comminication to server, execution, and returning the result
	KeepTokenOnAuthenticationInvalid bool    // if KeepTokenOnAuthenticationInvalid is true, the auth token isn't cleared on failure to authenticate
	SkipVerifyCertificate            bool    // if true, no certificate checking is done for https calls
	PrefixURL                        string  // Stores PrefixURL, so easy to compare if it needs changing

	gettingResources zmap.LockMap[string, bool]
	pollGetters      zmap.LockMap[string, func()]
	callURL          string
	ID               string
}

// client is structure to store received info from the call
type clientReceivePayload struct {
	Result                json.RawMessage
	Error                 string         `json:",omitempty"`
	TransportError        TransportError `json:",omitempty"`
	AuthenticationInvalid bool
}

const (
	dateHeaderID    = "X-Date"
	timeoutHeaderID = "X-Timeout-Secs"
)

// MainClient is the main, default client. Is set in zapp, and used in zusers.
var (
	MainClient          *Client
	registeredResources []string
	EnableLogClient     zlog.Enabler
)

// NewClient creates a client with a url prefix, adding zrest.AppURLPrefix
// This is
func NewClient(prefixURL string, id string) *Client {
	c := &Client{}
	c.PrefixURL = prefixURL
	c.callURL = zstr.Concat("/", prefixURL, zrest.AppURLPrefix, "zrpc")
	if id == "" {
		id = zstr.GenerateRandomHexBytes(12)
	}
	c.ID = id
	c.TimeoutSecs = 100
	return c
}

// // Copy makes a copy of a client, to alter timeout or other fields.
// // Avoid copying the struct instead, as it contains mutexes and rate limiters not meant to be used in two places.
// func (c *Client) Copy() *Client {
// 	var n Client
// 	n.callURL = c.callURL
// 	n.id = c.id
// 	n.AuthToken = c.AuthToken
// 	n.TimeoutSecs = c.TimeoutSecs
// 	n.KeepTokenOnAuthenticationInvalid = c.KeepTokenOnAuthenticationInvalid
// 	n.SkipVerifyCertificate = c.SkipVerifyCertificate
// 	return &n
// }

// Call is used to execute a remote call. method is Type.MethodName
// input can be nil if not used, and result can be nil if not used/not in method.
func (c *Client) Call(method string, input, result any) error {
	err, terr := c.callWithTransportError(method, c.TimeoutSecs, input, result)
	if terr != nil && err == nil {
		err = terr
	}
	return err
}

func (c *Client) callWithTransportError(method string, timeoutSecs float64, input, result any) (err error, terr error) {
	var rp clientReceivePayload
	cp := CallPayload{Method: method, Args: input}
	cp.ClientID = c.ID
	params := zhttp.MakeParameters()
	params.TimeoutSecs = c.TimeoutSecs
	params.SkipVerifyCertificate = c.SkipVerifyCertificate
	params.Headers[dateHeaderID] = time.Now().UTC().Format(ztime.JavascriptISO)
	params.Headers[timeoutHeaderID] = strconv.FormatFloat(timeoutSecs, 'f', -1, 64)
	if c.AuthToken != "" {
		cp.Token = c.AuthToken
	}
	urlArgs := map[string]string{
		"method": method,
	}
	surl, _ := zhttp.MakeURLWithArgs(c.callURL, urlArgs)
	// zlog.Warn("CALL:", surl)
	_, err = zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		return nil, zlog.Error(err, "post", surl)
	}
	if rp.AuthenticationInvalid { // check this first, will probably be an error also
		zlog.Info("zprc AuthenticationInvalid:", method, c.AuthToken, c.KeepTokenOnAuthenticationInvalid)
		if !c.KeepTokenOnAuthenticationInvalid {
			c.AuthToken = ""
		}
		rp.TransportError = "authentication invalid"
		if c.HandleAuthenticanFailedFunc != nil {
			c.HandleAuthenticanFailedFunc()
		}
	}
	if rp.TransportError != "" {
		return nil, rp.TransportError
	}
	if rp.Error != "" {
		return errors.New(rp.Error), nil
	}
	if !rp.AuthenticationInvalid && result != nil {
		err = json.Unmarshal(rp.Result, result)
		if err != nil {
			zlog.Error(err, c.AuthToken, "unmarshal")
			return nil, TransportError(err.Error())
		}
	}
	return nil, nil
}

// CallWithTimeout is a convenience method that calls method with a different timeout
func (c *Client) CallWithTimeout(timeoutSecs float64, method string, input, result any) error {
	err, terr := c.callWithTransportError(method, timeoutSecs, input, result)
	if terr != nil && err == nil {
		return terr
	}
	return err
}

func (c *Client) PollForUpdatedResources(got func(resID string)) {
	// zlog.Info("PollForUpdatedResources1")
	for _, r := range registeredResources {
		got(r)
		f, got := c.pollGetters.Get(r)
		// zlog.Info("PollForUpdatedResources", r, got)
		if got {
			f()
		}
	}
	ztimer.RepeatForever(1, func() {
		var resIDs []string
		err := c.Call("ZRPCResourceCalls.GetUpdatedResourcesAndSetSent", nil, &resIDs)
		if err != nil {
			zlog.Error(err, "updateResources err:")
			return
		}
		for _, s := range resIDs {
			if !zstr.StringsContain(registeredResources, s) {
				continue
			}
			setting, _ := c.gettingResources.Get(s)
			if setting {
				continue
			}
			c.gettingResources.Set(s, true)
			f, has := c.pollGetters.Get(s)
			if has {
				f()
			} else {
				got(s)
			}
			c.gettingResources.Set(s, false)
		}
	})
}

func (c *Client) CallGetForUpdatedResources(resIDs []string) {
	for _, s := range resIDs {
		if !zstr.StringsContain(registeredResources, s) {
			continue
		}
		setting, _ := c.gettingResources.Get(s)
		if setting {
			continue
		}
		c.gettingResources.Set(s, true)
		f, has := c.pollGetters.Get(s)
		if has {
			f()
		}
		c.gettingResources.Set(s, false)
	}
}

func RegisterResources(resources ...string) {
	registeredResources = zstr.UnionStringSet(registeredResources, resources)
}

func (c *Client) RegisterPollGetter(resID string, get func()) {
	// zlog.Info("RegisterPollGetter", resID)
	RegisterResources(resID)
	c.pollGetters.Set(resID, get)
}
