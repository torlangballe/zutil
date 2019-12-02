package zrpc

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	zrest "github.com/torlangballe/zutil/urest"

	"github.com/torlangballe/zutil/uhttp"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	rpcjson "github.com/gorilla/rpc/json"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/ustr"
	"github.com/torlangballe/zutil/zlog"
)

var ClientID string
var AuthToken string
var Local = false
var UseHttp = false
var Port = 1200
var ServerUsingAuthToken = false
var Address = "http://127.0.0.1"
var server *rpc.Server

// CallsBase is just something to create a type to add callable methods to
type CallsBase int
type Any struct{}

type Handler int

// Which clients have I sent info about resource being updated to

var updatedResourcesSentToClient = map[string]map[string]bool{}

type RPCCalls CallsBase

var Calls RPCCalls

func (c *RPCCalls) GetUpdatedResources(req *http.Request, args *Any, reply *[]string) error {
	client, err := AuthenticateRequest(req)
	if err != nil {
		return err
	}
	for res, m := range updatedResourcesSentToClient {
		if m[client] == false {
			*reply = append(*reply, res)
			m[client] = true
		}
	}
	return nil
}

func SetResourceUpdated(resID, byClientID string) {
	m := map[string]bool{}
	if byClientID != "" {
		m[byClientID] = true
	}
	updatedResourcesSentToClient[resID] = m
}

func ClearResourceUpdated(resID, clientID string) {
	if updatedResourcesSentToClient[resID] == nil {
		updatedResourcesSentToClient[resID] = map[string]bool{}
	}
	updatedResourcesSentToClient[resID][clientID] = true
}

var handler Handler

func (h Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	server.ServeHTTP(w, req)
}

func makeUrl() string {
	return fmt.Sprintf("%s:%d/rpc", Address, Port)
}
func doServeHTTP(w http.ResponseWriter, req *http.Request) {
	// fmt.Println("DoServeHTTP:", req.Method, req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token, X-ZUI-Client-Id, X-ZUI-Auth-Token")

	if req.Method == "OPTIONS" {
		return
	}
	server.ServeHTTP(w, req)
}

func InitClient() {
	ClientID = ustr.GenerateRandomHex(8)
}

func InitServer(router *mux.Router) (err error) {
	if !Local {
		fmt.Println("Serving HTTP RPC on Port", Port)
		go http.ListenAndServe(fmt.Sprintf(":%d", Port), router)
		if err != nil {
			return err
		}
		fmt.Println("Serving RPC on Port", Port)
		server = rpc.NewServer()
		server.RegisterCodec(rpcjson.NewCodec(), "application/json")
	}
	Register(Calls)
	zrest.AddHandle(router, "/rpc", doServeHTTP).Methods("POST", "OPTIONS")
	return
}

var registeredOwners = map[string]bool{}

func Register(owners ...interface{}) {
	for _, o := range owners {
		name := reflect.Indirect(reflect.ValueOf(o)).Type().Name()
		if registeredOwners[name] {
			zlog.Fatal(nil, "calls owner with same name exists:", name)
		}
		registeredOwners[name] = true
		fmt.Println("Register:", name)
		if !Local {
			server.RegisterService(o, "")
		}
	}
}

func CallRemote(method interface{}, args interface{}, reply interface{}) error {
	// https://github.com/golang/go/wiki/WebAssembly#configuring-fetch-options-while-using-nethttp
	if Local {
		fn := reflect.ValueOf(method)
		vargs := []reflect.Value{
			reflect.ValueOf(args),
			reflect.ValueOf(reply),
		}
		vals := fn.Call(vargs)
		if len(vals) == 1 {
			e, _ := vals[0].Interface().(error)
			return e
		}
		return errors.New("bad values returned")
	}

	surl := makeUrl()
	name, err := getRemoteCallName(method)
	if err != nil {
		return errors.Wrap(err, "call remote, call remote get name")
	}
	// fmt.Println("CALL:", name, args)

	message, err := rpcjson.EncodeClientRequest(name, args)
	if err != nil {
		return zlog.Error(err, "CallRemote encode client request")
	}
	headers := map[string]string{
		"X-ZUI-Client-Id":  ClientID,
		"X-ZUI-Auth-Token": AuthToken,
	}
	resp, _, err := uhttp.PostBytesSetContentLength(surl, "application/json", message, headers) //, message, map[string]string{
	// fmt.Println("POST RPC:", err, surl, string(message))
	// 	"js.fetch:mode": "no-cors",
	// })
	if err != nil {
		return zlog.Error(err, zlog.StackAdjust(1), "CallRemote post:", name)
	}
	defer resp.Body.Close()

	err = rpcjson.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return zlog.Error(err, "CallRemote decode")
	}
	return nil
}

func AuthenticateRequest(req *http.Request) (client string, err error) {
	clientID := req.Header.Get("X-ZUI-Client-Id")
	//token := req.Header.Get("X-ZUI-Auth-Token")
	if ServerUsingAuthToken {

	}
	return clientID, nil
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
	m := ustr.HeadUntilString(parts[1], "-")
	return obj + "." + m, nil
}
