package zrpc

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"github.com/torlangballe/zutil/urest"

	"github.com/torlangballe/zutil/uhttp"

	"github.com/gorilla/mux"
	"github.com/gorilla/rpc"
	rpcjson "github.com/gorilla/rpc/json"

	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/ustr"
	"github.com/torlangballe/zutil/zlog"
)

// var client *rpc.Client
var Local = false
var UseHttp = false
var Port = 1200

var server *rpc.Server

type Handler int

var handler Handler

func (h Handler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("ServeHTTP")
	// w.Header().Set("Access-Control-Allow-Origin", "[*]"))
	// w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
	// w.Header().Set("Access-Control-Allow-Credentials", "true")
	// w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token")
	server.ServeHTTP(w, req)
}

func makeUrl() string {
	return fmt.Sprintf("http://127.0.0.1:%d/rpc", Port)
}
func doServeHTTP(w http.ResponseWriter, req *http.Request) {
	fmt.Println("DoServeHTTP:", req.Method, req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Origin", req.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,PUT,OPTIONS")
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Headers", "Origin, X-Requested-With, Content-Type, Accept, Access-Token")

	if req.Method == "OPTIONS" {
		return
	}
	// if req.Method == "OPTIONS" {
	// 	return
	// }
	server.ServeHTTP(w, req)
}

func InitServer(router *mux.Router) (err error) {
	if !Local {
		//		corsObj := handlers.AllowedOrigins([]string{"*"})
		fmt.Println("Serving HTTP RPC on Port", Port)
		go http.ListenAndServe(fmt.Sprintf(":%d", Port), router)
		//		go http.ListenAndServe(fmt.Sprintf(":%d", Port), handlers.CORS(corsObj)(router))

		//		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", Port))
		if err != nil {
			return err
		}
		fmt.Println("Serving RPC on Port", Port)
		server = rpc.NewServer()
		server.RegisterCodec(rpcjson.NewCodec(), "application/json")
		//		server.RegisterCodec(rpcjson.NewCodec(), "application/json;charset=UTF-8")
		//		go http.Serve(listener, handler)
	}

	urest.AddHandle(router, "/rpc", doServeHTTP).Methods("POST", "OPTIONS")
	//	router.Handle("/rpc", handler.Method("POST"))
	return
}

func Register(rcvr interface{}) error {
	fmt.Println("Register!")
	if !Local {
		server.RegisterService(rcvr, "")
	}
	return nil
}

func CallRemote(method interface{}, args interface{}, reply interface{}) error {
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

	// https://github.com/golang/go/wiki/WebAssembly#configuring-fetch-options-while-using-nethttp
	surl := makeUrl()
	name, err := getRemoteCallName(method)
	if err != nil {
		return errors.Wrap(err, "call remote, call remote get name")
	}
	fmt.Println("CALL:", name, args)

	message, err := rpcjson.EncodeClientRequest(name, args)
	if err != nil {
		return zlog.Error(err, "CallRemote encode client request")
	}
	resp, _, err := uhttp.PostBytesSetContentLength(surl, "application/json", message, map[string]string{
		"js.fetch:mode": "no-cors",
	})
	fmt.Println("POST2:", err)
	//	resp, err := uhttp.PostBytesMakeError(surl, "application/json", message)
	if err != nil {
		return zlog.Error(err, "CallRemote post")
	}

	defer resp.Body.Close()

	err = rpcjson.DecodeClientResponse(resp.Body, &reply)
	if err != nil {
		return zlog.Error(err, "CallRemote decode")
	}
	return nil
}

func getRemoteCallName(method interface{}) (string, error) {
	// or get from interface: https://stackoverflow.com/questions/36026753/is-it-possible-to-get-the-function-name-with-reflect-like-this?noredirect=1&lq=1
	name := runtime.FuncForPC(reflect.ValueOf(method).Pointer()).Name()
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
	call := obj + "." + m
	//	fmt.Println("CallName:", n, parts, name, call)
	return call, nil
}
