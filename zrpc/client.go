package zrpc

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strings"

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zrest"

	rpcjson "github.com/gorilla/rpc/json"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

// Any is a dummy type when you don't care about a Call's in or out argument
type Any struct{}

type Client struct {
	ID        string
	AuthToken string
	//var UseHttp = false
	Port      int
	ToAddress string
}

var ToServerClient *Client
var ToNativeClient *Client

func NewClient() *Client {
	c := &Client{}
	c.ID = zstr.GenerateRandomHexBytes(8)
	c.Port = 1200
	c.ToAddress = "http://127.0.0.1"
	return c
}

func (c *Client) makeUrl() string {
	return fmt.Sprintf("%s:%d%srpc", c.ToAddress, c.Port, zrest.AppURLPrefix)
}

func (c *Client) SetAddressFromURL(surl string) {
	u, err := url.Parse(surl)
	zlog.AssertNotError(err, "href parse")
	c.ToAddress = u.Scheme + "://" + u.Hostname()
}

func (c *Client) CallRemote(method interface{}, args interface{}, reply interface{}, timeoutSecs ...float64) error {
	// defer zlog.LogRecoverAndExit()
	name, err := getRemoteCallName(method)
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "call remote get name")
	}
	return c.CallRemoteWithName(name, args, reply, timeoutSecs...)
}

func (c *Client) CallRemoteWithName(name string, args interface{}, reply interface{}, timeoutSecs ...float64) error {
	// https://github.com/golang/go/wiki/WebAssembly#configuring-fetch-options-while-using-nethttp

	// start := time.Now()
	surl := c.makeUrl()
	// zlog.Info("zrpc:", name, args)

	message, err := rpcjson.EncodeClientRequest(name, args)
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "call remote encode client request")
	}
	// fmt.Println("REMOTECALL2:", name, time.Since(start))
	params := zhttp.MakeParameters()
	params.UseHTTPS = false
	params.SkipVerifyCertificate = true
	params.Headers["X-ZUI-Client-Id"] = c.ID
	params.Headers["X-ZUI-Auth-Token"] = c.AuthToken
	params.Body = message
	params.ContentType = "application/json"
	params.Method = http.MethodPost
	if len(timeoutSecs) != 0 {
		params.TimeoutSecs = timeoutSecs[0]
	}
	resp, _, err := zhttp.SendBytesSetContentLength(surl, params) //, message, map[string]string{
	// zlog.Info("CallRemote:", name, surl, err)
	// 	"js.fetch:mode": "no-cors",
	// })
	// zlog.Info("POST RPC:", err, surl, zhttp.GetCopyOfResponseBodyAsString(resp))
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "call remote post:", name)
	}

	// sbody := zhttp.GetCopyOfResponseBodyAsString(resp)
	err = rpcjson.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return err
		//		return zlog.Error(err, zlog.StackAdjust(1), "call remote decode")
	}
	return nil
}

func getRemoteCallName(method interface{}) (string, error) {
	// or get from interface: https://stackoverflow.com/questions/36026753/is-it-possible-to-get-the-function-name-with-reflect-like-this?noredirect=1&lq=1
	rval := reflect.ValueOf(method)
	name := runtime.FuncForPC(rval.Pointer()).Name()

	parts := strings.Split(name, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	n := parts[len(parts)-1]
	parts = strings.Split(n, ".")
	if len(parts) > 3 || len(parts) < 2 {
		return "", errors.New("bad name extracted: " + n)
	}
	if len(parts) == 3 {
		parts = parts[1:]
	}
	obj := strings.Trim(parts[0], "()*")
	m := zstr.HeadUntil(parts[1], "-")
	return obj + "." + m, nil
}
