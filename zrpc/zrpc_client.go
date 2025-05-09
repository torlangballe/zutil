package zrpc

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zerrors"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// This is functionality to call rpc calls, using a client.

// Client is a type used to perform rpc calls
type Client struct {
	AuthToken                        string  // AuthToken is the token sent with the rpc call to authenticate or identify (for reverse calls)
	HandleAuthenticationFailedFunc   func()  // HandleAuthenticationFailedFunc is called if authentication fails
	TimeoutSecs                      float64 // A new client with a different timeout can be created. This is total time of comminication to server, execution, and returning the result
	KeepTokenOnAuthenticationInvalid bool    // if KeepTokenOnAuthenticationInvalid is true, the auth token isn't cleared on failure to authenticate
	SkipVerifyCertificate            bool    // if true, no certificate checking is done for https calls
	PrefixURL                        string  // Stores PrefixURL, so easy to compare if it needs changing
	ID                               string

	gettingResources zmap.LockMap[string, bool]
	pollGetters      zmap.LockMap[string, func()]
	callURL          string
	localExecutor    *Executor // if set, the call just calls method on executor instead
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

func NewClientWithLocalExecutor(e *Executor, id string) *Client {
	c := &Client{}
	c.localExecutor = e
	if id == "" {
		id = zstr.GenerateRandomHexBytes(12)
	}
	c.ID = id
	c.TimeoutSecs = 20
	return c
}

// NewClient creates a client with a url prefix, adding zrest.AppURLPrefix
// This is
func NewClient(prefixURL string, id string) *Client {
	c := &Client{}
	c.PrefixURL = prefixURL
	c.callURL = c.MakeCallURL("zrpc")
	if id == "" {
		id = zstr.GenerateRandomHexBytes(12)
	}
	// zlog.Info("zrpc.NewClient:", c.callURL)
	c.ID = id
	c.TimeoutSecs = 20
	return c
}

func (c *Client) MakeCallURL(name string) string {
	return zstr.Concat("/", c.PrefixURL, zrest.AppURLPrefix, name)
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

func (c *Client) doLocalCall(method string, timeoutSecs float64, input, result any) error {
	var ci ClientInfo
	// cp := callPayloadReceive{Method: method, Args: input, ClientID:c.ID}
	// cp := CallPayload{Method: method, Args: input}
	// cp.ClientID = c.ID
	// ci.Type = "zrpc"
	ci.ClientID = c.ID
	// ci.Token = token
	// ci.UserID = userID
	// ci.Request = req
	// ci.UserAgent = req.UserAgent()
	// ci.IPAddress = req.RemoteAddr
	// sdate := req.Header.Get(dateHeaderID)
	// stimeout := req.Header.Get(timeoutHeaderID)
	// timeoutSecs, _ := strconv.ParseFloat(stimeout, 64)
	// ci.SendDate, _ =  time.Parse(ztime.JavascriptISO, sdate)
	expires := time.Now().Add(ztime.SecondsDur(timeoutSecs))
	raw, err := json.Marshal(input)
	// expires := time.Now().Add(ztime.SecondsDur(timeoutSecs))
	lrp, err := c.localExecutor.callWithDeadline(ci, method, expires, raw, nil)
	if err == nil {
		return err
	}
	reflect.ValueOf(result).Elem().Set(reflect.ValueOf(lrp.Result))
	return nil
}

func (c *Client) callWithTransportError(method string, timeoutSecs float64, input, result any) (err error, terr error) {
	if c.localExecutor != nil {
		err = c.doLocalCall(method, timeoutSecs, input, result)
		return err, nil
	}
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
	dict := zdict.Dict{"RPC Method": method, "TimeoutSecs": timeoutSecs}
	// now := time.Now()
	_, err = zhttp.Post(surl, params, cp, &rp)
	if err != nil {
		limitID := zlog.Limit("zrpc.Post.Err." + method)
		zlog.Info(limitID, "zrpc Post", surl, err)
		return nil, zerrors.MakeContextError(dict, "Post to Remote URL", err)
	}
	if rp.AuthenticationInvalid { // check this first, will probably be an error also
		zlog.Info("zprc AuthenticationInvalid:", method, c.AuthToken, c.KeepTokenOnAuthenticationInvalid)
		if !c.KeepTokenOnAuthenticationInvalid {
			c.AuthToken = ""
		}
		rp.TransportError = "authentication invalid"
		if c.HandleAuthenticationFailedFunc != nil {
			c.HandleAuthenticationFailedFunc()
		}
	}
	if rp.TransportError != "" {
		return nil, zerrors.MakeContextError(dict, rp.TransportError)
	}
	if rp.Error != "" {
		return zerrors.MakeContextError(dict, errors.New(rp.Error)), nil
	}
	if !rp.AuthenticationInvalid && result != nil {
		err = json.Unmarshal(rp.Result, result)
		if err != nil {
			zlog.Error(c.AuthToken, "unmarshal", err)
			return nil, TransportError(err.Error())
		}
		// zlog.Info("zrpc.requestHTTPDataFields:", cp.Method, reflect.TypeOf(result))
		err = requestHTTPDataFields(result, c, "zrpc:"+method+":"+surl) // this is for getting big fields back as data. Doesn't work for sending though
		if err != nil {
			zlog.Error("requestHTTPDataFields", err)
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
			zlog.Error("updateResources err:", err)
			return
		}
		// zlog.Info("PollForUpdatedResources", resIDs, registeredResources)
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

func requestHTTPDataFields(s any, requestHTTPDataClient *Client, debugInfo string) error {
	var wg sync.WaitGroup
	outErr := new(error)
	rv := reflect.ValueOf(s)
	if rv.Kind() != reflect.Pointer || rv.Elem().Kind() != reflect.Struct {
		return nil
	}
	zreflect.ForEachField(s, zreflect.FlattenAll, func(each zreflect.FieldInfo) bool {
		parts, _ := zreflect.TagValuesForKey(each.StructField.Tag, "zrpc")
		// zlog.Info("zrpc.requestHTTPDataFields1:", each.StructField.Name)
		if zstr.StringsContain(parts, "http") {
			if !each.ReflectValue.CanSet() {
				zlog.Error("client: can't set zrpc:http field:", each.StructField.Name)
				return true
			}
			data, _ := each.ReflectValue.Interface().([]byte)
			if data == nil {
				*outErr = zlog.Error("client: field isn't []byte:", each.StructField.Name, debugInfo)
				return true
			}
			if len(data) == 0 {
				zlog.Info("requestHTTPDataFields: empty data:", each.StructField.Name, debugInfo)
				return true
			}
			if len(data) > 20 {
				zlog.Info("zrpc.requestHTTPDataFields: data too big, was it sent in []byte?", each.StructField.Name, debugInfo)
				return true
				// id := AddToTemporaryServe(each.ReflectValue.Bytes())
				// idBytes := []byte(strconv.FormatInt(id, 10))
				// each.ReflectValue.Set(reflect.ValueOf(idBytes))
			}
			str := string(data)
			id, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				*outErr = zlog.Error("client: id wasn't valid:", str, err, each.StructField.Name, debugInfo)
				return true
			}
			wg.Add(1)
			go func(id int64, rval reflect.Value) {
				reader, err := requestHTTPDataClient.RequestTemporaryServe(id)
				defer wg.Done()
				if err != nil {
					*outErr = zlog.Error("RequestTemporaryServe req err:", err, id, each.StructField.Name, debugInfo)
					return
				}
				buf, err := io.ReadAll(reader)
				if err != nil {
					*outErr = zlog.Error("RequestTemporaryServe read err:", err, id, each.StructField.Name, debugInfo)
					return
				}
				rval.SetBytes(buf)
				// zlog.Info("zrpc: Request http data field:", id, each.StructField.Name, len(buf))
			}(id, each.ReflectValue)
		}
		wg.Wait()
		return true
	})
	return *outErr
}

// RequestTemporaryServe requests to get data bytes with id set up with AddToTemporaryServe() in executor.
func (c *Client) RequestTemporaryServe(id int64) (io.ReadCloser, error) {
	params := zhttp.MakeParameters()
	params.Method = http.MethodGet
	params.TimeoutSecs = 20
	if c.AuthToken != "" {
		params.Headers["X-Token"] = c.AuthToken
	}
	args := map[string]string{"id": strconv.FormatInt(id, 10)}
	surl := c.MakeCallURL(tempDataMethod)
	surl, _ = zhttp.MakeURLWithArgs(surl, args)
	// zlog.Warn("CALL:", surl)
	resp, err := zhttp.GetResponse(surl, params)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
