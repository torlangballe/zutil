package zrpc

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"runtime"
	"strings"

	rpcjson "github.com/gorilla/rpc/json"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
)

// Any is a dummy type when you don't care about a Call's in or out argument
type Any struct{}

type Client struct {
	ID string
	//var UseHttp = false
	Port      int
	ToAddress string
}

const ClientIDKey = "ZRPC-Client-Id"

var ToServerClient *Client
var ToNativeClient *Client

func NewClient(useTokenAuth bool, port int) *Client {
	c := &Client{}
	if !useTokenAuth {
		c.ID = zstr.GenerateRandomHexBytes(8)
	}
	if port == 0 {
		port = 1200
	}
	c.Port = port
	c.ToAddress = "http://127.0.0.1"
	return c
}

func (c *Client) makeUrl() string {
	return fmt.Sprintf("%s:%d%srpc", c.ToAddress, c.Port, zrest.AppURLPrefix)
}

func (c *Client) SetAddressFromHost(scheme, address string) {
	c.ToAddress = scheme + "://" + address
	// zlog.Info("zrpc.SetAddressFromHost:", address)
}

func (c *Client) SetAddressFromURL(surl string) {
	u, err := url.Parse(surl)
	zlog.AssertNotError(err, "href parse")
	c.SetAddressFromHost(u.Scheme, u.Hostname())
}

func (c *Client) CallRemoteFunc(method interface{}, args interface{}, reply interface{}, timeoutSecs ...float64) error {
	// TODO: check that args and reply are the same as the 2 parameters in the actual method. (or nil?)
	name, err := getRemoteCallName(method)
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "call remote get name")
	}
	err = c.CallRemote(name, args, reply, timeoutSecs...)
	if err != nil {
		return err
	}
	// zlog.Info("Call:", name, err)
	return nil
}

func (c *Client) CallRemote(name string, args interface{}, reply interface{}, timeoutSecs ...float64) error {
	// https://github.com/golang/go/wiki/WebAssembly#configuring-fetch-options-while-using-nethttp

	// start := time.Now()
	surl := c.makeUrl()
	// zlog.Info("zrpc:", name, args)

	message, err := rpcjson.EncodeClientRequest(name, args)
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "call remote encode client request")
	}
	// if strings.Contains(name, "GetEvents") {
	// 	fmt.Println("REMOTECALL2:", name, string(message))
	// }
	params := zhttp.MakeParameters()
	params.UseHTTPS = false
	params.SkipVerifyCertificate = true
	params.Headers[ClientIDKey] = c.ID
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
		zlog.Error(err, zlog.StackAdjust(2), "call remote post:", name)
		return err
	}

	// sbody := zhttp.GetCopyOfResponseBodyAsString(resp)
	err = rpcjson.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		zlog.Error(err, zlog.StackAdjust(1), "call remote decode")
		return err
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
