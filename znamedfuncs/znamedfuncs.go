package znamedfuncs

import (
	"context"
	"encoding/json"
	"go/token"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type CallerOwner interface {
	GetCallerInfo() CallerInfo
}

// CallerInfo has information about the caller of a namedfuncs function and can be an optional first argument to a namedfuncs function.
type CallerInfo struct {
	CallerID          string          // CallerID identifies the caller
	Token             string          `json:",omitempty"` // Token can be any token, or a authentication token needed to allow the call
	Context           context.Context `json:"-"`
	TimeToLiveSeconds float64         `json:",omitempty"`
}

type Executor struct {
	Authenticator znet.TokenAuthenticator // used to authenticate a token comming from caller
	callMethods   map[string]*methodType  // stores all registered types/methods
	ErrorHandler  func(err error)         // calls this with errors that happen, for logging etc in system that uses namedfuncs
}

type CallPayloadSend struct {
	CallerInfo
	Method string
	Args   any
}

type CallPayloadReceive struct {
	CallerInfo
	Method string
	Args   json.RawMessage
}

// TransportError is a specific error type. Any problem with the actual business logic of a namedfuncs call, not something the called function does.
// returned as is, so we can check if it's an error returned from the call, or a problem calling.
type TransportError string

type Unused struct{} // Any is used in function definition arg when argument is not used. Result can be skipped.

// methodType stores the information of each method of each registered type
type methodType struct {
	Receiver      reflect.Value
	Method        reflect.Method
	hasClientInfo bool
	ArgType       reflect.Type
	ReplyType     reflect.Type
	AuthNotNeeded bool
}

// ReceivePayload is what the result of the call is returned in.
type ReceivePayload struct {
	Result         json.RawMessage
	Error          string         `json:",omitempty"`
	TransportError TransportError `json:",omitempty"`
	// AuthenticationInvalid bool
}

var (
	ExecuteTimedOutError       = TransportError("Execution timed out")
	AuthenticationInvalidError = TransportError("Authentication Invalid")
	EnableLogExecute           zlog.Enabler
)

func (c CallerInfo) GetCallerInfo() CallerInfo {
	return c
}

func (t TransportError) Error() string {
	return string(t)
}

func NewExecutor() *Executor {
	e := &Executor{}
	e.callMethods = map[string]*methodType{}
	// e.IPAddressWhitelist = map[string]bool{}
	return e
}

func (e *Executor) Error(parts ...any) error {
	err := zlog.Error(parts...)
	if e.ErrorHandler != nil {
		e.ErrorHandler(err)
	}
	return err
}

// Register registers instances of types that have methods in them suitable for being an namedfuncs call.
// func (t type) Method(<ci ClientInfo>, input, <*result>) error
func (e *Executor) Register(callers ...any) {
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

// suitableMethods returns methods on c's type that can be called (see Register())
func suitableMethods(c any) map[string]*methodType {
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
			hasClientInfo = (mtype.In(i) == reflect.TypeOf(CallerInfo{}) || mtype.In(i) == reflect.TypeOf(&CallerInfo{}))
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

func callMethod(e *Executor, ci CallerInfo, mtype *methodType, rawArg json.RawMessage, rp *ReceivePayload) {
	// zlog.Info("callMethod:", mtype.Method.Name)
	start := time.Now()
	defer func() {
		if time.Since(start) > time.Second*2 && mtype.Method.Name != "ReversePoll" { // ReversePoll waits for some result, so can take time on purpose
			zlog.Info("🟪Warning: Slow namedfuncs excute:", mtype.Method.Name, time.Since(start))
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
	err := json.Unmarshal(rawArg, argv.Interface())
	if err != nil {
		str := zstr.Spaced("Unmarshal:", err, mtype.Method, argv.Kind(), argv.Type(), zlog.Full(*mtype))
		rp.TransportError = TransportError(str)
		return
	}
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

	if ci.Context == nil {
		ci.Context = context.Background()
	}
	called := time.Now()
	completed := zprocess.RunFuncUntilContextDone(ci.Context, func() {
		returnValues = mtype.Method.Func.Call(args)
	})
	if !completed {
		str := zstr.Spaced("namedfuncs.Call expired before call", mtype.Method.Name, "since start/before http-fields:", time.Since(start), "since exe:", time.Since(called))
		rp.TransportError = TransportError(str)
		return
	}
	errInter := returnValues[0].Interface()
	if errInter != nil {
		err := errInter.(error)
		zlog.Error(EnableLogExecute, "Call Error", mtype.Method.Name, err)
		rp.Error = err.Error()
		return
	}
	if hasReply {
		rp.Result, err = json.Marshal(replyv.Elem().Interface())
		if err != nil {
			rp.TransportError = TransportError(zstr.Spaced("Marshal reply:", err, mtype.Method.Name))
		}
		return
	}
}

func (e *Executor) SetAuthNotNeededForMethod(name string) {
	// zlog.Info("SetAuthNotNeededForMethod:", e != nil, name)
	e.callMethods[name].AuthNotNeeded = true
}

func (e *Executor) Execute(cp *CallPayloadReceive, rp *ReceivePayload) {
	// defer zdebug.RecoverFromPanic(false, "")
	if e.Authenticator != nil && e.methodNeedsAuth(cp.Method) {
		var valid bool
		valid, _ = e.Authenticator.IsTokenValid(cp.Token)
		if !valid {
			zlog.Error("token not valid: '"+cp.Token+"'", zlog.Full(e.Authenticator), cp.CallerInfo)
			rp.TransportError = AuthenticationInvalidError
			return
		}
	}
	for n, m := range e.callMethods {
		if n == cp.Method {
			callMethod(e, cp.CallerInfo, m, cp.Args, rp)
			return
		}
	}
	rp.TransportError = TransportError("no method registered: " + cp.Method + " " + zlog.Full(cp.CallerInfo))
}

func (e *Executor) ExecuteFromToJSON(payload []byte, result *[]byte, ci CallerInfo) error {
	var cp CallPayloadReceive
	var rp ReceivePayload
	err := json.Unmarshal(payload, &cp)
	if err != nil {
		rp.TransportError = TransportError(err.Error())
	} else {
		if cp.TimeToLiveSeconds > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), ztime.SecondsDur(cp.TimeToLiveSeconds))
			defer cancel()
			cp.Context = ctx
		}
		e.Execute(&cp, &rp)
	}
	*result, err = json.Marshal(rp)
	if err != nil {
		zlog.Error("encode namedfuncs result", cp.Method, rp, err, zdebug.CallingStackString())
		return err
	}
	return nil
}
