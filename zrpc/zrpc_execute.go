package zrpc

import (
	"context"
	"encoding/json"
	"go/token"
	"reflect"

	"github.com/torlangballe/zutil/zlog"
)

var callMethods = map[string]*methodType{} // stores all registered types/methods
// Register registers instances of types that have methods in them suitable for being an rpc call.
// func (t type) Method(<ci ClientInfo>, input, <*result>) error
func Register(callers ...interface{}) {
	for _, c := range callers {
		methods := suitableMethods(c)
		for n, m := range methods {
			// zlog.Info("Reg:", n, m.Method)
			callMethods[n] = m
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
			hasClientInfo = reflect.TypeOf(ClientInfo{}) == mtype.In(i)
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

func methodNeedsAuth(name string) bool {
	m, got := callMethods[name]
	if !got {
		zlog.Error(nil, "methodNeedsAuth on unknown:", name)
		return true
	}
	return !m.AuthNotNeeded
}

func callMethod(ctx context.Context, ci ClientInfo, mtype *methodType, rawArg json.RawMessage) (rp receivePayload, err error) {
	// zlog.Info("callMethod:", mtype.Method.Name)
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
	args := []reflect.Value{mtype.Receiver}
	if mtype.hasClientInfo {
		args = append(args, reflect.ValueOf(ci))
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
	returnValues := mtype.Method.Func.Call(args)
	errInter := returnValues[0].Interface()
	if errInter != nil {
		err := errInter.(error)
		zlog.Error(err, "Call Error", mtype.Method)
		rp.Error = err.Error()
		return rp, nil
	}
	if hasReply {
		rp.Result = replyv.Interface()
	}
	return rp, nil
}

func callMethodName(ctx context.Context, ci ClientInfo, name string, rawArg json.RawMessage) (rp receivePayload, err error) {
	for n, m := range callMethods {
		if n == name {
			return callMethod(ctx, ci, m, rawArg)
		}
	}
	return rp, zlog.NewError("no method registered:", name)
}