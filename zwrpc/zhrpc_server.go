package zwrpc

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

func InitHTTPServer(router *mux.Router) {
	zrest.AddHandler(router, "xrpc", doServeHTTP).Methods("POST", "OPTIONS")
	// Register(Calls)
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
	defer zlog.HandlePanic(false)
	decoder := json.NewDecoder(req.Body)
	err := decoder.Decode(&cp)
	if err != nil {
		rp.TransportError = err.Error()
	} else {
		ctx := context.Background()
		rp, err = callMethodName(ctx, cp.Method, cp.Args)
		if err != nil {
			rp.Error = err.Error()
		}
	}
	encoder := json.NewEncoder(w)
	err = encoder.Encode(rp)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	// zlog.Info("zrpc.Serve done:", body)
}
