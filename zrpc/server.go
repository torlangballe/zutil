package zrpc

import (
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	rpcjson "github.com/gorilla/rpc/json"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

// CallsBase is just something to create a type to add callable methods to
type CallsBase int
type RPCCalls CallsBase

//var ServerUsingAuthToken = false
var ServerPort int = 1200
var server *rpc.Server
var registeredOwners = map[string]bool{}

// updatedResourcesSentToClient stores Which clients have I sent info about resource being updated to [res][client]bool
var updatedResourcesSentToClient = map[string]map[string]bool{}
var updatedResourcesMutex sync.Mutex

func InitServer(router *mux.Router, port int) error {
	//	go http.ListenAndServeTLS(fmt.Sprintf(":%d", ServerPort), "https/server.crt", "https/server.key", router)
	if port == 0 {
		port = 1200
	}
	ServerPort = port
	server = rpc.NewServer()
	registeredOwners = map[string]bool{}
	go http.ListenAndServe(fmt.Sprintf(":%d", ServerPort), router)
	zlog.Info("Serve RPC On:", ServerPort)
	server.RegisterCodec(rpcjson.NewCodec(), "application/json")
	zrest.AddHandler(router, "/rpc", doServeHTTP).Methods("POST", "OPTIONS")
	return nil
}

func doServeHTTP(w http.ResponseWriter, req *http.Request) {
	// TODO: See how little of this we can get away with
	// zlog.Info("DoServeHTTP:", req.Method, req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token, X-ZUI-Client-Id, X-ZUI-Auth-Token")

	defer req.Body.Close()

	if req.Method == "OPTIONS" {
		return
	}
	// zlog.Info("zrpc.doServeHTTP:", req.Method, uhttp.GetCopyOfRequestBodyAsString(req))
	server.ServeHTTP(w, req)
}

func Register(owners ...interface{}) {
	owners = append(owners, Calls)
	for _, o := range owners {
		name := reflect.Indirect(reflect.ValueOf(o)).Type().Name()
		if registeredOwners[name] {
			zlog.Fatal(nil, "calls owner with same name exists:", name)
		}
		registeredOwners[name] = true
		err := server.RegisterService(o, "")
		if err != nil {
			zlog.Error(err, zlog.StackAdjust(1), "rpc.Register")
		}
	}
}

func AuthenticateRequest(req *http.Request) (client string, err error) {
	clientID := req.Header.Get("X-ZUI-Client-Id")
	//token := req.Header.Get("X-ZUI-Auth-Token")
	// if ServerUsingAuthToken {

	// }
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
	updatedResourcesMutex.Lock()
	if updatedResourcesSentToClient[resID] == nil {
		updatedResourcesSentToClient[resID] = map[string]bool{}
	}
	updatedResourcesSentToClient[resID][clientID] = true
	updatedResourcesMutex.Unlock()
}

func (c *RPCCalls) GetUpdatedResources(req *http.Request, args *Any, reply *[]string) error {
	clientID, err := AuthenticateRequest(req)
	if err != nil {
		return err
	}
	*reply = []string{}
	updatedResourcesMutex.Lock()
	for res, m := range updatedResourcesSentToClient {
		if m[clientID] == false {
			*reply = append(*reply, res)
			m[clientID] = true
		}
	}
	updatedResourcesMutex.Unlock()
	// zlog.Info("GetUpdatedResources", *reply)
	return nil
}

var Calls = new(RPCCalls)
