package zrpc

import (
	"context"
	"encoding/json"
	"go/token"
	"net/http"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
)

type TokenAuthenticator interface {
	IsTokenValid(token string, req *http.Request) (bool, int64)
}

type Executor struct {
	Authenticator      TokenAuthenticator     // used to authenticate a token in a RPC call
	callMethods        map[string]*methodType // stores all registered types/methods
	IPAddressWhitelist map[string]bool        // if non-empty, only ip-addresses in map are allowed to be called from
	ErrorHandler       func(err error)        // calls this with errors that happen, for logging etc in system that uses zrpc
}

var EnableLogExecute zlog.Enabler

func NewExecutor() *Executor {
	e := &Executor{}
	e.callMethods = map[string]*methodType{}
	e.IPAddressWhitelist = map[string]bool{}
	return e
}

func (e *Executor) Error(parts ...any) error {
	err := zlog.Error(parts...)
	if e.ErrorHandler != nil {
		e.ErrorHandler(err)
	}
	return err
}

// Register registers instances of types that have methods in them suitable for being an rpc call.
// func (t type) Method(<ci ClientInfo>, input, <*result>) error
func (e *Executor) Register(callers ...interface{}) {
	for _, c := range callers {
		methods := suitableMethods(c)
		for n, m := range methods {
			_, got := e.callMethods[n]
			if got {
				e.Error("Registering existing call object:", n)
				break
			}
			// zlog.Info("REG:", n, m.Method, zlog.Pointer(e))
			e.callMethods[n] = m
		}
	}
}

// suitableMethods returns methods on c's type that can be called by zrpc (see Register())
func suitableMethods(c interface{}) map[string]*methodType {
	typeOfError := reflect.TypeOf((*error)(nil)).Elem()
	rval := reflect.ValueOf(c)
	t := rval.Type()
	et := t
	if rval.Kind() == reflect.Ptr {
		et = rval.Elem().Type()
	}
	pre := et.Name() + "."
	methods := make(map[string]*methodType)
	for m := 0; m < t.NumMethod(); m++ {
		var hasClientInfo bool
		method := t.Method(m)
		mtype := method.Type
		mname := pre + method.Name
		if method.PkgPath != "" {
			continue
		}
		i := 1
		if mtype.NumIn() > 2 {
			hasClientInfo = reflect.TypeOf(ClientInfo{}) == mtype.In(i) || reflect.TypeOf(&ClientInfo{}) == mtype.In(i)
			if hasClientInfo {
				i++
			}
		}
		if mtype.NumIn() < i+1 {
			zlog.Info("Register: method", mname, "has", mtype.NumIn(), "input parameters; wrong amount:", mtype.NumIn()-1)
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(i)
		if !isExportedOrBuiltinType(argType) {
			zlog.Info("Register: argument type of method", mname, "is not exported:", argType)
			continue
		}
		i++
		var replyType reflect.Type
		if mtype.NumIn() > i { // Second arg must be a pointer or interface.
			replyType = mtype.In(i)
			if replyType.Kind() != reflect.Ptr && replyType.Kind() != reflect.Interface {
				zlog.Info("Register: reply type of method", mname, "is not a pointer:", replyType, method.Func.CanAddr())
				continue
			}
			if !isExportedOrBuiltinType(replyType) { // Reply type must be exported.
				zlog.Info("Register: reply type of method", mname, "is not exported:", replyType)
				continue
			}
		}
		if mtype.NumOut() != 1 { // Method needs one out.
			zlog.Info("Register: method", mname, "has", mtype.NumOut(), "output parameters; needs exactly one")
			continue
		}
		if returnType := mtype.Out(0); returnType != typeOfError { // The return type of the method must be error.
			zlog.Info("Register: return type of method", mname, "is", returnType, ", must be error")
			continue
		}
		methods[mname] = &methodType{Receiver: rval, Method: method, ArgType: argType, ReplyType: replyType, hasClientInfo: hasClientInfo}
	}
	return methods
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return token.IsExported(t.Name()) || t.PkgPath() == "" // PkgPath will be non-empty even for an exported type, so we need to check the type name as well.
}

func (e *Executor) methodNeedsAuth(name string) bool {
	m, got := e.callMethods[name]
	if !got {
		e.Error("methodNeedsAuth: Couldn't find method:", name)
		return true
	}
	return !m.AuthNotNeeded
}

func (e *Executor) callMethod(ctx context.Context, ci ClientInfo, mtype *methodType, rawArg json.RawMessage, requestHTTPDataClient *Client) (rp receivePayload, err error) {
	// zlog.Info("callMethod:", mtype.Method.Name)
	start := time.Now()
	defer func() {
		if time.Since(start) > time.Second*2 && mtype.Method.Name != "ReversePoll" { // ReversePoll waits for some result, so can take time on purpose
			zlog.Info("🟪Warning: Slow zrpc excute:", mtype.Method.Name, time.Since(start), err)
		}
	}()
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
		zlog.Error("Unmarshal:", err, mtype.Method, argv.Kind(), argv.Type(), zlog.Full(*mtype))
		return rp, err
	}

	if requestHTTPDataClient != nil { // this is for getting big args in a reverse rpc call
		// zlog.Info("zrpc.requestHTTPDataFields in revcall:", mtype.Method, reflect.TypeOf(argv.Interface()))
		requestHTTPDataFields(argv.Interface(), requestHTTPDataClient, "zrpc:"+mtype.Method.Name+":"+ci.IPAddress)
	}

	// zlog.Info("zrpc.CallMethod:", mtype.Method)
	if argIsValue {
		argv = argv.Elem()
	}
	args := []reflect.Value{mtype.Receiver}
	if mtype.hasClientInfo {
		args = append(args, reflect.ValueOf(&ci))
	}
	args = append(args, argv)
	hasReply := mtype.ReplyType != nil
	if hasReply {
		switch mtype.ReplyType.Elem().Kind() {
		case reflect.Map:
			replyv = reflect.New(mtype.ReplyType.Elem())
			replyv.Elem().Set(reflect.MakeMap(mtype.ReplyType.Elem()))
		case reflect.Slice:
			replyv = reflect.New(mtype.ReplyType.Elem())
			replyv.Elem().Set(reflect.MakeSlice(mtype.ReplyType.Elem(), 0, 0))
		default:
			replyv = reflect.New(mtype.ReplyType.Elem())
		}
		args = append(args, replyv)
	}
	var returnValues []reflect.Value
	// deadline, _ := ctx.Deadline()
	// zlog.Info("Call with deadline:", mtype.Method.Name, -time.Since(deadline))

	called := time.Now()
	completed := zprocess.RunFuncUntilContextDone(ctx, func() {
		returnValues = mtype.Method.Func.Call(args)
	})
	if !completed {
		return rp, zlog.NewError("zrpc.Call expired before call", mtype.Method.Name, "since start/before http-fields:", time.Since(start), "since exe:", time.Since(called))
	}

	errInter := returnValues[0].Interface()
	if errInter != nil {
		err := errInter.(error)
		zlog.Error(EnableLogExecute, "Call Error", mtype.Method.Name, err)
		rp.Error = err.Error()
		return rp, nil
	}
	if hasReply {
		rp.Result = replyv.Interface()
	}
	return rp, nil
}

func (e *Executor) callMethodName(ctx context.Context, ci ClientInfo, name string, rawArg json.RawMessage, requestHTTPDataClient *Client) (rp receivePayload, err error) {
	for n, m := range e.callMethods {
		// zlog.Info("callMethName:", n, name, n == name)
		if n == name {
			return e.callMethod(ctx, ci, m, rawArg, requestHTTPDataClient)
		}
	}
	return rp, zlog.NewError("no method registered:", name, ci.Request.RemoteAddr)
}

func (e *Executor) callWithDeadline(ci ClientInfo, method string, expires time.Time, args json.RawMessage, requestHTTPDataClient *Client) (receivePayload, error) {
	var rp receivePayload
	var err error
	// zlog.Info("zrpc callWithDeadline:", method, zlog.Pointer(e))
	var from string
	if requestHTTPDataClient != nil {
		from += "from: " + requestHTTPDataClient.callURL
	}
	if time.Since(expires) >= 0 {
		zlog.Error("zrpc Executor: callWithDeadline expired before execute:", expires, from)
		rp.TransportError = TransportError(zstr.Spaced("Call received after timeout.", method, time.Since(expires)))
	} else {
		ctx, cancel := context.WithDeadline(context.Background(), expires)
		defer cancel()
		ci.Context = ctx
		rp, err = e.callMethodName(ctx, ci, method, args, requestHTTPDataClient)
		// zlog.Info("zrpc callWithDeadline: callMethod done:", method, err, method, zlog.Pointer(e))
		if err != nil {
			if !zdebug.IsInTests {
				zlog.Error("callWithDeadline execute error:", expires, from, err)
			}
			rp.Error = err.Error()
		}
		deadline, ok := ctx.Deadline()
		// zlog.Warn("callWithDeadline:", method, expires, deadline, ok)
		if ok && time.Since(deadline) >= 0 {
			rp.TransportError = ExecuteTimedOutError
		}
	}
	return rp, err
}
