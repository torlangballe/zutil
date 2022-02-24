package xrpc

import (
	"context"
	"encoding/json"
	"go/token"
	"net/http"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zlog"
)

type callPayload struct {
	Token string
	//	Args       json.RawMessage
	Args       interface{}
	ToID       string
	Method     string
	InstanceID int64
}

type receivePayload struct {
	Result         interface{}
	Error          string
	TransportError string
	InstanceID     int64
}

type methodType struct {
	Receiver  reflect.Value
	Method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

const (
	runMethodSecs = 30
	pollSecs      = 20
)

var (
	callMethods = map[string]*methodType{}
	typeOfError = reflect.TypeOf((*error)(nil)).Elem()
)

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return token.IsExported(t.Name()) || t.PkgPath() == "" // PkgPath will be non-empty even for an exported type, so we need to check the type name as well.
}

func suitableMethods(c interface{}) map[string]*methodType {
	rval := reflect.ValueOf(c)
	t := rval.Type()
	et := t
	if rval.Kind() == reflect.Ptr {
		et = rval.Elem().Type()
	}
	pre := et.PkgPath() + "." + et.Name() + "."
	methods := make(map[string]*methodType)
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		mname := pre + method.Name
		// zlog.Info("Register:", mname)
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 3 {
			zlog.Error(nil, "Register: method", mname, "has", mtype.NumIn(), "input parameters; needs exactly three")
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(1)
		if !isExportedOrBuiltinType(argType) {
			zlog.Error(nil, "Register: argument type of method", mname, "is not exported:", argType)
			continue
		}
		// Second arg must be a pointer.
		replyType := mtype.In(2)
		if replyType.Kind() != reflect.Ptr {
			zlog.Error(nil, "Register: reply type of method", mname, "is not a pointer:", replyType)
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			zlog.Error(nil, "Register: reply type of method", mname, "is not exported:", replyType)
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			zlog.Error(nil, "Register: method", mname, "has", mtype.NumOut(), "output parameters; needs exactly one")
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			zlog.Error(nil, "Register: return type of method", mname, "is", returnType, ", must be error")
			continue
		}
		methods[mname] = &methodType{Receiver: rval, Method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods
}

func Register(callers ...interface{}) {
	for _, c := range callers {
		methods := suitableMethods(c)
		for n, m := range methods {
			callMethods[n] = m
		}
	}
}

func callMethod(ctx context.Context, mtype *methodType, args interface{}) (rp *receivePayload, err error) { // rawArg json.RawMessage
	var replyv reflect.Value
	// var argv, replyv reflect.Value
	// argIsValue := false // if true, need to indirect before calling.
	// if mtype.ArgType.Kind() == reflect.Ptr {
	// 	argv = reflect.New(mtype.ArgType.Elem())
	// } else {
	// 	argv = reflect.New(mtype.ArgType) // argv guaranteed to be a pointer now.
	// 	argIsValue = true
	// }
	// err = json.Unmarshal(rawArg, argv.Interface())
	// if err != nil {
	// 	zlog.Error(err, "UMARSH ERR:", argv.Kind())
	// 	return rp, err
	// }
	// if argIsValue {
	// 	argv = argv.Elem()
	// }
	replyv = reflect.New(mtype.ReplyType.Elem())
	argv := reflect.ValueOf(args)
	switch mtype.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mtype.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mtype.ReplyType.Elem(), 0, 0))
	}

	function := mtype.Method.Func
	returnValues := function.Call([]reflect.Value{mtype.Receiver, argv, replyv})
	errInter := returnValues[0].Interface()
	rp = &receivePayload{}
	if errInter != nil {
		err := errInter.(error)
		zlog.Error(err, "Call Error")
		rp.Error = err.Error()
		return rp, nil
	}
	rp.Result = replyv.Interface()
	return rp, nil
}

func callName(ctx context.Context, name string, args interface{}) (rp *receivePayload, err error) { // args json.RawMessage
	for n, m := range callMethods {
		if n == name {
			return callMethod(ctx, m, args)
		}
	}
	return rp, zlog.NewError("no method registered:", name)
}

func handleCallWithMethod(cp *callPayload) (rp *receivePayload, err error) {
	ctx := context.Background()
	// zlog.Info("start incomingWS")
	callCtx, wcancel := context.WithTimeout(ctx, time.Second*10)
	defer wcancel()
	zlog.Info("CALLED:", cp.Method, cp.Args)
	rp, err = callName(callCtx, cp.Method, cp.Args)
	if err != nil {
		zlog.Error(err, "call")
		rp.TransportError = err.Error()
	}
	return
}

func handleCallWithMethodWriteBack(w http.ResponseWriter, cp *callPayload) error {
	rp, err := handleCallWithMethod(cp)
	if err != nil {
		return err
	}
	e := json.NewEncoder(w)
	err = e.Encode(rp)
	if err != nil {
		return zlog.Error(err, "encode")
	}
	return nil
}
