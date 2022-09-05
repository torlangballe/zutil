package zrpc2

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

type CallsBase bool
type RPCCalls CallsBase
type TokenAuthenticator interface {
	IsTokenValid(token string) bool
}

var (
	Calls                        = new(RPCCalls)
	updatedResourcesSentToClient = map[string]map[string]bool{}
	updatedResourcesMutex        sync.Mutex
	authenticator                TokenAuthenticator
)

func InitServer(router *mux.Router, a TokenAuthenticator) {
	authenticator = a
	zrest.AddHandler(router, "xrpc", doServeHTTP).Methods("POST", "OPTIONS")
	Register(Calls)
}

func doServeHTTP(w http.ResponseWriter, req *http.Request) {
	var cp callPayloadReceive
	var rp receivePayload
	var token string
	// TODO: See how little of this we can get away with
	// zlog.Info("zrpc.DoServeHTTP:", req.Method, "from:", req.Header.Get("Origin"), req.URL)

	zrest.AddCORSHeaders(w, req)
	// w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	// w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
	// w.Header().Set("Access-Control-Allow-Credentials", "true")
	// w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token, X-ZUI-Client-Id, X-ZUI-Auth-Token")

	defer req.Body.Close()
	if req.Method == "OPTIONS" {
		return
	}

	// body := zhttp.GetCopyOfRequestBodyAsString(req)
	defer zlog.HandlePanic(false)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&cp)
	// zlog.Info("CP:", body, err)
	call := true
	if err != nil {
		rp.TransportError = err.Error()
		call = false
	} else {
		if methodNeedsAuth(cp.Method) && authenticator != nil {
			token = req.Header.Get("X-Token")
			if !authenticator.IsTokenValid(token) {
				zlog.Error(nil, "token not valid:", token)
				rp.TransportError = "authentication error"
				rp.AuthenticationInvalid = true
				call = false
			}
		}
		if call {
			ctx := context.Background()
			var ci ClientInfo
			ci.ClientID = cp.ClientID
			ci.Token = token
			ci.UserAgent = req.UserAgent()
			ci.IPAddress = req.RemoteAddr
			rp, err = callMethodName(ctx, ci, cp.Method, cp.Args)
			if err != nil {
				zlog.Error(err, "call")
				rp.Error = err.Error()
			}
		}
	}
	// b, _ := json.Marshal(rp)
	// zlog.Info("Called2:", cp.Method, rp)

	encoder := json.NewEncoder(w)
	err = encoder.Encode(rp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// zlog.Info("zrpc.Serve done", err, rp)
}

func (c *RPCCalls) GetUpdatedResourcesAndSetSent(ci ClientInfo, args *Unused, reply *[]string) error {
	// fmt.Println("GetUpdatedResourcesAndSetSent", clientID)
	// zlog.Info("GetUpdatedResourcesAndSetSent", clientID, updatedResourcesSentToClient)
	*reply = []string{}
	updatedResourcesMutex.Lock()
	for res, m := range updatedResourcesSentToClient {
		if !m[ci.ClientID] {
			*reply = append(*reply, res)
			m[ci.ClientID] = true
		}
	}
	updatedResourcesMutex.Unlock()
	// zlog.Info("GetUpdatedResources Got", *reply)
	return nil
}
