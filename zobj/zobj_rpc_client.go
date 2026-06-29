package zobj

import (
	"encoding/json"
	"reflect"

	"github.com/torlangballe/zutil/xrpc"
	"github.com/torlangballe/zutil/zlog"
)

var RPCCaller xrpc.Callable

/*
func CallGetSlicesForUserAndType[S any](slice *[]S, ask GetRowsAsk) error {
	var rows []Row
	err := xrpc.MainCaller().Call("ZOBJCalls.GetRowsForUserAndType", ask, &rows)
	if err != nil {
		return err
	}
	for _, row := range rows {
		var s S
		rowToObject(&row, s)
		*slice = append(*slice, s)
	}
	return nil
}
*/

func CallSelect[S any](slices *[]S, forUserID int64, where string) error {
	var ask SelectAsk
	var jsonData []byte
	var s S

	_, st, err := LookupStructTableFromStruct(s)
	zlog.OnError(err, reflect.TypeOf(s))
	// _, st, err := LookupStructTable(objType)
	// if zlog.OnError(err, objType) {
	// 	return err
	// }
	ask.Table = st.Table
	ask.TypeName = st.typeName
	ask.Where = where
	err = RPCCaller.Call("ZOBJCalls.Select", ask, &jsonData)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonData, slices)
	if err != nil {
		return err
	}
	return nil
}

func CallInsert[S any](s S, forUserID int64) (int64, error) {
	var ask InsertAsk
	var id int64
	var err error

	_, st, err := LookupStructTableFromStruct(s)
	zlog.AssertNotError(err)
	ask.TypeName = st.typeName
	ask.Table = st.Table
	ask.ObjectAsJSON, err = json.Marshal(s)
	ask.ForUserID = forUserID
	if err != nil {
		return 0, err
	}
	err = RPCCaller.Call("ZOBJCalls.Insert", ask, &id)
	if err != nil {
		return 0, err
	}
	return id, nil
}

func CallUpdate[S any](s S, forUserID int64, where string) error {
	var ask UpdateAsk
	var err error

	_, st, err := LookupStructTableFromStruct(s)
	if err != nil {
		return err
	}
	zlog.AssertNotError(err)
	ask.TypeName = st.typeName
	ask.Table = st.Table
	ask.ObjectAsJSON, err = json.Marshal(s)
	ask.ForUserID = forUserID
	ask.Where = where
	err = RPCCaller.Call("ZOBJCalls.Update", ask, nil)
	if err != nil {
		return err
	}
	return nil
}
