package zwrpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/token"
	"io"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type callPayload struct {
	Method     string
	Args       interface{}
	IDWildcard string
}

type callPayloadReceive struct {
	Method     string
	Args       json.RawMessage
	IDWildcard string
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

type Any struct{} // Any is used in function definition args/result when argument is not used

var (
	callMethods    = map[string]*methodType{}
	typeOfError    = reflect.TypeOf((*error)(nil)).Elem()
	TransportError = errors.New("transport error")
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
	pre := et.Name() + "." // et.PkgPath() + "." +
	methods := make(map[string]*methodType)
	for m := 0; m < t.NumMethod(); m++ {
		method := t.Method(m)
		mtype := method.Type
		mname := pre + method.Name
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if mtype.NumIn() != 3 {
			zlog.Info("Register: method", mname, "has", mtype.NumIn(), "input parameters; needs exactly three")
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(1)
		if !isExportedOrBuiltinType(argType) {
			zlog.Info("Register: argument type of method", mname, "is not exported:", argType)
			continue
		}
		// Second arg must be a pointer or interface.
		replyType := mtype.In(2)
		if replyType.Kind() != reflect.Ptr && replyType.Kind() != reflect.Interface {
			zlog.Info("Register: reply type of method", mname, "is not a pointer:", replyType, method.Func.CanAddr())
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			zlog.Info("Register: reply type of method", mname, "is not exported:", replyType)
			continue
		}
		// Method needs one out.
		if mtype.NumOut() != 1 {
			zlog.Info("Register: method", mname, "has", mtype.NumOut(), "output parameters; needs exactly one")
			continue
		}
		// The return type of the method must be error.
		if returnType := mtype.Out(0); returnType != typeOfError {
			zlog.Info("Register: return type of method", mname, "is", returnType, ", must be error")
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
		zlog.Error(err, "Unmarshal:", argv.Kind())
		return rp, err
	}
	if argIsValue {
		argv = argv.Elem()
	}

	var a *Any
	hasReply := mtype.ReplyType != reflect.TypeOf(a)
	// zlog.Info("Type:", mtype.ReplyType, reflect.TypeOf(a), hasReply)
	if hasReply {
		replyv = reflect.New(mtype.ReplyType.Elem())

		switch mtype.ReplyType.Elem().Kind() {
		case reflect.Map:
			replyv.Elem().Set(reflect.MakeMap(mtype.ReplyType.Elem()))
		case reflect.Slice:
			replyv.Elem().Set(reflect.MakeSlice(mtype.ReplyType.Elem(), 0, 0))
		}
	} else {
		replyv = reflect.ValueOf(a)
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
	if hasReply {
		rp.Result = replyv.Interface()
	}
	return rp, nil
}

func callMethodName(ctx context.Context, name string, rawArg json.RawMessage) (rp receivePayload, err error) {
	for n, m := range callMethods {
		if n == name {
			return callMethod(ctx, m, rawArg)
		}
	}
	return rp, zlog.NewError("no method registered:", name)
}

func readCallPayload(c *websocket.Conn) (cp *callPayloadReceive, dbytes []byte, err error) {
	buffer := bytes.NewBuffer([]byte{})
	ctx := context.Background()
	zlog.Info("readCallPayload")
	mtype, r, err := c.Reader(ctx)
	zlog.Info("readCallPayload done", err)
	if err != nil {
		zlog.Error(err, "make reader", mtype)
		return nil, nil, err
	}
	zlog.Info("mtype:", mtype)
	n, err := buffer.ReadFrom(r)
	if err != nil {
		return nil, nil, zlog.Error(err, "readfrom", n)
	}
	cp = &callPayloadReceive{}
	dbytes = buffer.Bytes()
	err = json.Unmarshal(dbytes, cp)
	if err != nil {
		return nil, nil, zlog.Error(err, "unmarshal", cp != nil, n, string(dbytes), zlog.CallingStackString())
	}
	return
}

func handleIncomingCall(c *Client, cp *callPayloadReceive) {
	ctx := context.Background()
	zlog.Info("start incomingWS", c != nil, cp != nil)
	wctx, wcancel := context.WithTimeout(ctx, time.Second*10)
	defer wcancel()
	rp, err := callMethodName(ctx, cp.Method, cp.Args)
	if err != nil {
		rp.TransportError = err.Error()
	}
	wsjson.Write(wctx, c.ws, rp)
}

func SendReceiveDataToWS(id string, ws *websocket.Conn, bytes []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	writer, err := ws.Writer(ctx, websocket.MessageText)
	if err != nil {
		zlog.Error(err, "Error getting writer:", id)
		return nil, err
	}
	n, err := writer.Write(bytes)
	if err != nil {
		zlog.Error(err, "Error copying to client server:", id, n)
		return nil, err
	}
	zlog.Info("SendReceiveDataToWS")
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel2()
	_, reader, err := ws.Reader(ctx2)
	if err != nil {
		zlog.Error(err, "get reader:", id)
		return nil, err
	}
	return io.ReadAll(reader)
}
