//go:build server

package zrpc

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
	"github.com/torlangballe/zutil/ztime"
)

// TokenAuthenticator is used to authenticate a token in ClientInfo, can be zuser doing it, or whatever.
// InitServer initializes a rpc server with an authenticator, and registers RPCCalls,
// which has the build-in rpc methods for resources and reverse-calls.
func NewServer(router *mux.Router, a TokenAuthenticator) *Executor {
	e := NewExecutor()
	// zlog.Info("zrpc.NewServer e:", zlog.Pointer(e))
	e.Authenticator = a
	zrest.AddHandler(router, "zrpc", e.doServeHTTP).Methods("POST", "OPTIONS")
	//!!! e.Register(e)
	return e
}

// SetAuthNotNeededForMethod is used to exclude methods from needing authentication.
// Login methods that create a token for example.
func (e *Executor) SetAuthNotNeededForMethod(name string) {
	// zlog.Info("SetAuthNotNeededForMethod:", e != nil, name)
	e.callMethods[name].AuthNotNeeded = true
}

// doServeHTTP responds to a /zrpc request. It gets the method and arguments by parsing the json body.
// Note that the method name in url is only for debugging.
// The method is found in callMethods, called, and results/errors returned in the response to the request.
func (e *Executor) doServeHTTP(w http.ResponseWriter, req *http.Request) {
	var cp callPayloadReceive
	var rp receivePayload
	var token string

	// zlog.Warn("zrpc.doServeHTTP:", req.URL.String(), zlog.Pointer(e))
	// defer zlog.Info("zrpc.doServeHTTP DONE:", req.URL.Path, req.URL.Query())
	zrest.AddCORSHeaders(w, req)
	defer req.Body.Close()
	if req.Method == "OPTIONS" {
		return
	}
	defer zlog.HandlePanic(false)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&cp)
	call := true
	if err != nil {
		rp.TransportError = TransportError(err.Error())
		call = false
	} else {
		token = cp.Token
		if e.Authenticator != nil && e.methodNeedsAuth(cp.Method) {
			if !e.Authenticator.IsTokenValid(token) {
				zlog.Error(nil, "token not valid: '"+token+"'", req.RemoteAddr, req.URL.Path, req.URL.Query())
				rp.TransportError = "authentication error"
				rp.AuthenticationInvalid = true
				call = false
			}
		}
		if call && len(e.IPAddressWhitelist) > 0 {
			if !e.IPAddressWhitelist[req.RemoteAddr] {
				err := zlog.NewError("zrpc.Call", cp.Method, "calling ip not in whitelist", req.RemoteAddr, e.IPAddressWhitelist)
				rp.TransportError = TransportError(err.Error())
				rp.AuthenticationInvalid = true
				zlog.Error(err)
				call = false
			}
		}
		if call {
			var ci ClientInfo
			ci.Type = "zrpc"
			ci.ClientID = cp.ClientID
			ci.Token = token
			ci.UserAgent = req.UserAgent()
			ci.IPAddress = req.RemoteAddr
			sdate := req.Header.Get(dateHeaderID)
			stimeout := req.Header.Get(timeoutHeaderID)
			timeoutSecs, _ := strconv.ParseFloat(stimeout, 64)
			ci.SendDate, _ = time.Parse(ztime.JavascriptISO, sdate)
			expires := time.Now().Add(ztime.SecondsDur(timeoutSecs))
			rp, err = e.callWithDeadline(ci, cp.Method, expires, cp.Args)
		}
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(rp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
