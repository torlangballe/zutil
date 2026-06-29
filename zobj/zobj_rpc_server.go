//go:build server

package zobj

import (
	"encoding/json"
	"reflect"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
)

type ZOBJCalls struct{}

// type Commands struct {
// 	ztermfields.SliceCommander
// }

// var (
// 	CommandsNode Commands
// )

func (ZOBJCalls) Select(ci *znamedfuncs.ClientInfo, ask SelectAsk, rowsJSON *[]byte) error {
	rtype, st, err := LookupStructTable(ask.TypeName)
	if zlog.OnError(err, ask.TypeName) {
		return err
	}
	slice := reflect.MakeSlice(reflect.SliceOf(rtype), 0, 10)
	snew := reflect.New(slice.Type())
	snew.Elem().Set(slice)

	//	err = selectIntoSlice(sliceRVal.Addr().Interface(), objPtrRVal.Interface(), st, rtype, ask.ObjType, ask.MakeLinkCountItems, ask.Where, ask.ForUserID, ci.UserID)
	// ask.MakeLinkCountItems where false is below
	err = selectIntoSlice(snew.Interface(), st, rtype, ask.Where, ask.ForUserID, ci.UserID, false)
	zlog.Info("SELECT", ask.TypeName, snew.Type(), err, snew.Elem().Len())
	data, err := json.Marshal(snew.Interface())
	if err != nil {
		return err
	}
	*rowsJSON = data
	return nil
}

func (ZOBJCalls) Insert(ci *znamedfuncs.ClientInfo, ask InsertAsk, id *int64) error {
	rtype, st, err := LookupStructTable(ask.TypeName)
	if zlog.OnError(err, ask.TypeName) {
		return err
	}

	objPtrRVal := reflect.New(rtype)
	err = json.Unmarshal(ask.ObjectAsJSON, objPtrRVal.Interface())
	if err != nil {
		return err
	}
	zlog.Info("rpc Insert:", ci.UserID)
	*id, err = insertFromStruct(objPtrRVal.Interface(), st, rtype, ask.ForUserID, ci.UserID)
	if err != nil {
		return err
	}
	return nil
}

func (ZOBJCalls) Update(ci *znamedfuncs.ClientInfo, ask UpdateAsk) error {
	rtype, st, err := LookupStructTable(ask.TypeName)
	if zlog.OnError(err, ask.TypeName) {
		return err
	}

	objPtrRVal := reflect.New(rtype)
	err = json.Unmarshal(ask.ObjectAsJSON, objPtrRVal.Interface())
	if err != nil {
		return err
	}
	err = updateFromStruct(objPtrRVal.Interface(), st, rtype, ask.ForUserID, ci.UserID, ask.Where)
	if err != nil {
		return err
	}
	return nil
}
