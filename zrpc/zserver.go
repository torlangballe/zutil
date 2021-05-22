// +build server

package zrpc

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/torlangballe/zutil/zhttp"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	rpcjson "github.com/gorilla/rpc/json"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zrest"
)

// CallsBase is just something to create a type to add callable methods to
type CallsBase int
type RPCCalls CallsBase

//var ServerUsingAuthToken = false
var (
	ServerPort       int = 1200
	server           *rpc.Server
	registeredOwners = map[string]bool{}

	// updatedResourcesSentToClient stores Which clients have I sent info about resource being updated to [res][client]bool
	updatedResourcesSentToClient = map[string]map[string]bool{}
	updatedResourcesMutex        sync.Mutex
)

func InitServer(router *mux.Router, port int, certFilesSuffix string) error {
	//	go http.ListenAndServeTLS(fmt.Sprintf(":%d", ServerPort), "https/server.crt", "https/server.key", router)
	if port == 0 {
		port = 1200
	}
	ServerPort = port
	server = rpc.NewServer()
	registeredOwners = map[string]bool{}
	if certFilesSuffix != "" {
		go znet.ServeHTTPS(ServerPort, certFilesSuffix, router)
	} else {
		go http.ListenAndServe(fmt.Sprintf(":%d", ServerPort), router)
	}
	zlog.Info("🟨Serve RPC On:", ServerPort)
	server.RegisterCodec(rpcjson.NewCodec(), "application/json")
	zrest.AddHandler(router, "rpc", doServeHTTP).Methods("POST", "OPTIONS")
	return nil
}

func doServeHTTP(w http.ResponseWriter, req *http.Request) {
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
	// defer func() {
	// 	zlog.Info("Defer")
	// 	err := zlog.HandlePanic(false)
	// 	if err != nil {
	// 		http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	}
	// }()
	server.ServeHTTP(w, req)
}

func Register(owners ...interface{}) {
	owners = append(owners, Calls)
	for _, o := range owners {
		name := reflect.Indirect(reflect.ValueOf(o)).Type().Name()
		if registeredOwners[name] {
			zlog.Fatal(nil, "calls owner with same name exists:", name)
		}
		// zlog.Info("zrpc.Register name:", name)
		registeredOwners[name] = true
		err := server.RegisterService(o, "")
		if err != nil {
			zlog.Error(err, zlog.StackAdjust(1), "rpc.Register")
		}
	}
}

func AuthenticateRequest(req *http.Request) (client string, err error) {
	clientID := req.Header.Get(ClientIDKey)
	return clientID, nil
}

func SetResourceUpdated(resID, byClientID string) {
	// zlog.Info("SetResourceUpdated:", resID) //, "\n", zlog.GetCallingStackString())
	m := map[string]bool{}
	if byClientID != "" {
		m[byClientID] = true
	}
	updatedResourcesMutex.Lock()
	updatedResourcesSentToClient[resID] = m
	updatedResourcesMutex.Unlock()
}

func ClearResourceUpdated(resID, clientID string) {
	// zlog.Info("ClearResourceUpdated:", resID, clientID) //, "\n", zlog.GetCallingStackString())
	updatedResourcesMutex.Lock()
	if updatedResourcesSentToClient[resID] == nil {
		updatedResourcesSentToClient[resID] = map[string]bool{}
	}
	updatedResourcesSentToClient[resID][clientID] = true
	updatedResourcesMutex.Unlock()
}

func (c *RPCCalls) GetUpdatedResources(req *http.Request, args *Any, reply *[]string) error {
	// zlog.Info("GetUpdatedResources")
	clientID, err := AuthenticateRequest(req)
	if err != nil {
		return err
	}
	// zlog.Info("GetUpdatedResources", clientID, updatedResourcesSentToClient)
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

// GetURL is a convenience function to get the contents of a url via the server.
func (c *RPCCalls) GetURL(req *http.Request, surl *string, reply *[]byte) error {
	params := zhttp.MakeParameters()
	_, err := zhttp.Get(*surl, params, reply)
	return err
}

var Calls = new(RPCCalls)
