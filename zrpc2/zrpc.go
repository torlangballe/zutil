package zrpc2

import (
	"context"
	"encoding/json"
	"go/token"
	"reflect"

	"github.com/torlangballe/zutil/zlog"
)

type callPayload struct {
	ClientID   string
	Method     string
	Args       interface{}
	IDWildcard string
}

// callPayloadReceive is received. It has to have same named fields as callPayload
type callPayloadReceive struct {
	ClientID   string
	Method     string
	Args       json.RawMessage
	IDWildcard string
}

type receivePayload struct {
	Result                any
	Error                 string
	TransportError        string
	AuthenticationInvalid bool
}

type methodType struct {
	Receiver      reflect.Value
	Method        reflect.Method
	hasClientInfo bool
	ArgType       reflect.Type
	ReplyType     reflect.Type
	AuthNotNeeded bool
}

type ClientInfo struct {
	ClientID  string
	Token     string
	UserAgent string
	IPAddress string
}

type TransportError struct {
	Text string
}

func (t *TransportError) Error() string {
	return t.Text
}

type Unused struct{} // Any is used in function definition args/result when argument is not used

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
	pre := et.Name() + "." // et.PkgPath() + "." +
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
		if mtype.NumIn() == 4 {
			hasClientInfo = true
			clientType := mtype.In(1)
			zlog.Assert(clientType == reflect.TypeOf(ClientInfo{}), mname, clientType, reflect.TypeOf(ClientInfo{}))
			i++
		} else if mtype.NumIn() != 3 {
			zlog.Info("Register: method", mname, "has", mtype.NumIn(), "input parameters; needs exactly three")
			continue
		}
		// First arg need not be a pointer.
		argType := mtype.In(i)
		if !isExportedOrBuiltinType(argType) {
			zlog.Info("Register: argument type of method", mname, "is not exported:", argType)
			continue
		}
		// Second arg must be a pointer or interface.
		replyType := mtype.In(i + 1)
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
		// zlog.Info("REG:", mname, argType, replyType, hasClientID)
		methods[mname] = &methodType{Receiver: rval, Method: method, ArgType: argType, ReplyType: replyType, hasClientInfo: hasClientInfo}
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

func SetMethodAuthNotNeeded(name string) {
	callMethods[name].AuthNotNeeded = true
}

func methodNeedsAuth(name string) bool {
	m, got := callMethods[name]
	if !got {
		zlog.Error(nil, "methodNeedsAuth on unknown:", name)
		return true
	}
	return !m.AuthNotNeeded
}

func callMethod(ctx context.Context, ci ClientInfo, mtype *methodType, rawArg json.RawMessage) (rp receivePayload, err error) {
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
		zlog.Error(err, "Unmarshal:", mtype.Method, argv.Kind(), argv.Type(), zlog.Full(*mtype))
		return rp, err
	}
	if argIsValue {
		argv = argv.Elem()
	}
	var u *Unused
	hasReply := mtype.ReplyType != reflect.TypeOf(u)
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
	} else {
		replyv = reflect.ValueOf(u)
	}
	// zlog.Info("Type:", mtype.ReplyType.Elem().Kind(), mtype.ReplyType, reflect.TypeOf(u), replyv)
	function := mtype.Method.Func
	args := []reflect.Value{mtype.Receiver}
	if mtype.hasClientInfo {
		args = append(args, reflect.ValueOf(ci))
	}
	args = append(args, argv, replyv)
	// zlog.Info("callMethod:", mtype.ArgType, mtype.Method.Name, args, mtype.hasClientInfo)
	returnValues := function.Call(args)
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
	// b, _ := json.Marshal(rp)
	// zlog.Info("Called:", mtype.Method.Name, hasReply, mtype.Method.Name, string(b), rp)
	return rp, nil
}

func callMethodName(ctx context.Context, ci ClientInfo, name string, rawArg json.RawMessage) (rp receivePayload, err error) {
	for n, m := range callMethods {
		if n == name {
			// zlog.Info("METH:", zlog.Full(m))
			return callMethod(ctx, ci, m, rawArg)
		}
	}
	return rp, zlog.NewError("no method registered:", name)
}

// func readCallPayload(c *websocket.Conn) (cp *callPayloadReceive, dbytes []byte, err error) {
// 	buffer := bytes.NewBuffer([]byte{})
// 	ctx := context.Background()
// 	// zlog.Info("readCallPayload")
// 	mtype, r, err := c.Reader(ctx)
// 	// zlog.Info("readCallPayload done", err)
// 	if err != nil {
// 		zlog.Error(err, "make reader", mtype)
// 		return nil, nil, err
// 	}
// 	// zlog.Info("mtype:", mtype)
// 	n, err := buffer.ReadFrom(r)
// 	if err != nil {
// 		return nil, nil, zlog.Error(err, "readfrom", n)
// 	}
// 	cp = &callPayloadReceive{}
// 	dbytes = buffer.Bytes()
// 	err = json.Unmarshal(dbytes, cp)
// 	if err != nil {
// 		return nil, nil, zlog.Error(err, "unmarshal", cp != nil, n, string(dbytes), zlog.CallingStackString())
// 	}
// 	return
// }
