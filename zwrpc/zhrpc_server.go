package zwrpc

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

type CallsBase int
type RPCCalls CallsBase

var (
	Calls                        = new(RPCCalls)
	updatedResourcesSentToClient = map[string]map[string]bool{}
	updatedResourcesMutex        sync.Mutex
)

func InitHTTPServer(router *mux.Router) {
	zrest.AddHandler(router, "xrpc", doServeHTTP).Methods("POST", "OPTIONS")
	Register(Calls)
}

func doServeHTTP(w http.ResponseWriter, req *http.Request) {
	var cp callPayloadReceive
	var rp receivePayload
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
	if err != nil {
		rp.TransportError = err.Error()
	} else {
		ctx := context.Background()
		rp, err = callMethodName(ctx, cp.ClientID, cp.Method, cp.Args)
		if err != nil {
			zlog.Error(err, "call")
			rp.Error = err.Error()
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

func (c *RPCCalls) GetUpdatedResourcesAndSetSent(clientID string, args *Unused, reply *[]string) error {
	// fmt.Println("GetUpdatedResourcesAndSetSent", clientID)
	// zlog.Info("GetUpdatedResourcesAndSetSent", clientID, updatedResourcesSentToClient)
	*reply = []string{}
	updatedResourcesMutex.Lock()
	for res, m := range updatedResourcesSentToClient {
		if m[clientID] == false {
			*reply = append(*reply, res)
			m[clientID] = true
		}
	}
	updatedResourcesMutex.Unlock()
	// zlog.Info("GetUpdatedResources Got", *reply)
	return nil
}
