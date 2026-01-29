//go:build server

package zrpc

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zdebug"
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
	zrest.AddHandler(router, TempDataMethod, e.doServeTempDataHTTP).Methods("GET", "OPTIONS")
	return e
}

// SetAuthNotNeededForMethod is used to exclude methods from needing authentication.
// Login methods that create a token for example.
func (e *Executor) SetAuthNotNeededForMethod(name string) {
	// zlog.Info("SetAuthNotNeededForMethod:", e != nil, name)
	e.callMethods[name].AuthNotNeeded = true
}

func (e *Executor) doServeTempDataHTTP(w http.ResponseWriter, req *http.Request) {
	zrest.AddCORSHeaders(w, req)
	if req.Method == http.MethodOptions {
		return
	}
	defer func() {
		io.Copy(io.Discard, req.Body)
		req.Body.Close()
	}()
	if req.Method == "OPTIONS" {
		return
	}
	id := zrest.GetInt64Val(req.URL.Query(), "id", 0)
	if id == 0 {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "doServeTempDataHTTP: no id")
		return
	}
	ztoken := req.Header.Get("X-Token")
	if e.Authenticator != nil {
		valid, _ := e.Authenticator.IsTokenValid(ztoken, req)
		if !valid {
			zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "doServeTempDataHTTP auth error:", ztoken)
			return
		}
	}
	data, got := temporaryDataServe.Get(id)
	if !got {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "doServeTempDataHTTP: No data available for id:", id)
		return
	}
	n, err := w.Write(data)
	if err != nil || n < len(data) {
		zrest.ReturnAndPrintError(w, req, http.StatusInternalServerError, "doServeTempDataHTTP: No data available for id: ", id, n)
		return
	}
}

// doServeHTTP responds to a /zrpc request. It gets the method and arguments by parsing the json body.
// Note that the method name in url is only for debugging.
// The method is found in callMethods, called, and results/errors returned in the response to the request.
func (e *Executor) doServeHTTP(w http.ResponseWriter, req *http.Request) {
	var cp callPayloadReceive
	var rp receivePayload
	var token string
	var userID int64

	// zlog.Warn("zrpc.doServeHTTP:", req.URL.String(), req.Method)
	// defer zlog.Info("zrpc.doServeHTTP DONE:", req.URL.Path, req.URL.Query())
	zrest.AddCORSHeaders(w, req)
	if req.Method == http.MethodOptions {
		return
	}
	defer func() {
		io.Copy(io.Discard, req.Body)
		req.Body.Close()
	}()
	if req.Method == "OPTIONS" {
		return
	}
	defer zdebug.RecoverFromPanic(false, "")
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&cp)
	call := true
	if err != nil {
		rp.TransportError = TransportError(err.Error())
		call = false
	} else {
		token = cp.Token
		if e.Authenticator != nil && e.methodNeedsAuth(cp.Method) {
			var valid bool
			valid, userID = e.Authenticator.IsTokenValid(token, req)
			if !valid {
				zlog.Error("token not valid: '"+token+"'", zlog.Full(e.Authenticator), req.RemoteAddr, req.URL.Path, req.URL.Query())
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
			ci.UserID = userID
			ci.Request = req
			ci.UserAgent = req.UserAgent()
			ci.IPAddress = req.RemoteAddr
			sdate := req.Header.Get(dateHeaderID)
			stimeout := req.Header.Get(timeoutHeaderID)
			timeoutSecs, _ := strconv.ParseFloat(stimeout, 64)
			ci.SendDate, _ = time.Parse(ztime.JavascriptISO, sdate)
			expires := time.Now().Add(ztime.SecondsDur(timeoutSecs))
			rp, err = e.callWithDeadline(ci, cp.Method, expires, cp.Args, nil)
			if rp.Result != nil && err == nil {
				// registerHTTPDataFields(rp.Result)
			}
		}
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(rp)
	if err != nil {
		zlog.Error("encode rpc result", cp.Method, rp, err, zdebug.CallingStackString())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}


