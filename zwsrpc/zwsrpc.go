package zwsrpc

import (
	"context"
	"encoding/json"
	"errors"
	"go/token"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type callPayload struct {
	Token string
	Name  string
	Arg   interface{}
}

type callPayloadReceive struct {
	Token string
	Name  string
	Arg   json.RawMessage
}

type receivePayload struct {
	Result         interface{}
	Error          string
	TransportError string
}

type methodType struct {
	Receiver  reflect.Value
	Method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
}

var (
	callMethods    = map[string]*methodType{}
	typeOfError    = reflect.TypeOf((*error)(nil)).Elem()
	TransportError = errors.New("transport error")
)

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	// PkgPath will be non-empty even for an exported type,
	// so we need to check the type name as well.
	return token.IsExported(t.Name()) || t.PkgPath() == ""
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

func callMethod(ctx context.Context, mtype *methodType, rawArg json.RawMessage) (rp receivePayload, err error) {
	var argv, replyv reflect.Value
	argIsValue := false // if true, need to indirect before calling.
	if mtype.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(mtype.ArgType.Elem())
	} else {
		argv = reflect.New(mtype.ArgType) // argv guaranteed to be a pointer now.
		argIsValue = true
	}
	err = json.Unmarshal(rawArg, argv.Interface())
	if err != nil {
		zlog.Error(err, "UMARSH ERR:", argv.Kind())
		return rp, err
	}
	if argIsValue {
		argv = argv.Elem()
	}
	replyv = reflect.New(mtype.ReplyType.Elem())

	switch mtype.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(mtype.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(mtype.ReplyType.Elem(), 0, 0))
	}

	function := mtype.Method.Func
	returnValues := function.Call([]reflect.Value{mtype.Receiver, argv, replyv})
	errInter := returnValues[0].Interface()
	if errInter != nil {
		err := errInter.(error)
		zlog.Error(err, "Call Error")
		rp.Error = err.Error()
		return rp, nil
	}
	rp.Result = replyv.Interface()
	return rp, nil
}

func callName(ctx context.Context, name string, rawArg json.RawMessage) (rp receivePayload, err error) {
	for n, m := range callMethods {
		if n == name {
			return callMethod(ctx, m, rawArg)
		}
	}
	return rp, zlog.NewError("no method registered:", name)
}

func handleIncoming(c *websocket.Conn) {
	ctx := context.Background()
	// zlog.Info("start incomingWS")
	for {
		var cp callPayloadReceive
		err := wsjson.Read(ctx, c, &cp)
		// zlog.Info("incoming read", err, cp)
		if err != nil {
			zlog.Error(err)
			c.Close(websocket.StatusInternalError, err.Error())
			return
		}
		wctx, wcancel := context.WithTimeout(ctx, time.Second*10)
		defer wcancel()
		// zlog.Info("CALL:", cp.Name, cp.Arg)
		rp, err := callName(ctx, cp.Name, cp.Arg)
		if err != nil {
			zlog.Error(err)
			rp.TransportError = err.Error()
		}
		wsjson.Write(wctx, c, rp)
	}
}
