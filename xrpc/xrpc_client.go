package xrpc

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

type Client struct {
	ID        string
	Port      int
	ToAddress string
}

const ClientIDKey = "ZRPC-Client-Id"

var DefaultClient *Client

func NewClient(useTokenAuth bool, port int) *Client {
	c := &Client{}
	if !useTokenAuth {
		c.ID = zstr.GenerateRandomHexBytes(8)
	}
	if port == 0 {
		port = 1200
	}
	c.Port = port
	c.ToAddress = "https://127.0.0.1"
	return c
}

func (c *Client) makeUrl(postfix string) string {
	var sport string
	if c.Port != 0 {
		sport = fmt.Sprintf(":%d", c.Port)
	}
	return fmt.Sprintf("%s:%s%srpc%s", c.ToAddress, sport, zrest.AppURLPrefix, postfix)
}

func (c *Client) SetAddressFromHost(scheme, address string) {
	c.ToAddress = scheme + "://" + address
	// zlog.Info("zrpc.SetAddressFromHost:", address)
}

func (c *Client) SetAddressFromURL(surl string) {
	u, err := url.Parse(surl)
	zlog.AssertNotError(err, "href parse")
	u.Host = zstr.HeadUntil(u.Host, ":")
	c.ToAddress = u.String()
}

func (c *Client) makePostParameters(timeoutSecs float64) zhttp.Parameters {
	params := zhttp.MakeParameters()
	params.UseHTTPS = true
	params.SkipVerifyCertificate = true
	params.Headers[ClientIDKey] = c.ID
	params.Method = http.MethodPost
	params.TimeoutSecs = timeoutSecs
	return params
}

func (c *Client) CallRemote(method string, args interface{}, reply interface{}, toID string) error {
	surl, err := zhttp.MakeURLWithArgs(c.makeUrl(""), map[string]string{"method": method})
	if err != nil {
		return zlog.Error(err, "make url", method)
	}
	var cp callPayload
	var rp receivePayload
	cp.Args = args
	cp.ToID = toID
	params := c.makePostParameters(runMethodSecs + 5)
	_, err = zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		zlog.Error(err, "call remote post:", method)
		return err
	}
	return nil
}

func (c *Client) PollForReceived(id string) {
	for {
		var cp callPayload
		surl := c.makeUrl("Poll")
		params := zhttp.MakeParameters()
		params.TimeoutSecs = pollSecs + 2
		_, err := zhttp.Get(surl, params, &cp)
		if err != nil {
			zlog.Error(err, "get")
			continue
		}
		go c.handleGotMethodToCall(&cp)
	}
}

func (c *Client) handleGotMethodToCall(cp *callPayload) {
	rp, err := handleCallWithMethod(cp)
	if err != nil {
		zlog.Error(err, "call method on poll", cp.Method)
		return
	}
	args := map[string]string{"method": cp.Method} // we add method as argument to url for debugging purposes
	surl, err := zhttp.MakeURLWithArgs(c.makeUrl("Receive"), args)
	if err != nil {
		zlog.Error(err, "make url", cp.Method)
		return
	}
	rp.InstanceID = cp.InstanceID
	params := c.makePostParameters(10)
	_, err = zhttp.Post(surl, params, rp, nil)
	if err != nil {
		zlog.Error(err, "call remote post:", cp.Method)
		return
	}
}
