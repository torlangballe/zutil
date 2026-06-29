//go:build zui

package zobj

import (
	"errors"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zslicegrid"
	"github.com/torlangballe/zutil/xrpc"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zreflect"
	"github.com/torlangballe/zutil/zusers"
)

type TableOwner[S any] struct {
	Table            *zslicegrid.TableView[S]
	TypeName         string
	TableOptions     zslicegrid.OptionType
	Slice            []S
	Where            string
	GetAndUpdateFunc func()
	gotten           bool
}

func (t *TableOwner[S]) Init() {
	var s S

	typeName := zreflect.MakeTypeNameWithType(s)
	if typeName == "" {
		zalert.ShowError(errors.New("zobj.TableOwner: Type has no name"))
		return
	}
	// zlog.Info("TO.Init", typeName, zlog.Pointer(t))

	if t.GetAndUpdateFunc == nil {
		// zlog.Info("TO.Init2", t.ObjectType, zlog.Pointer(t))
		t.GetAndUpdateFunc = func() {
			// zlog.Info("zobj.Table.GetAndUpdateFunc", typeName, zlog.Pointer(t))
			go func() {
				err := t.CallGet()
				if err != nil {
					t.gotten = true
				}
			}()
		}
	}
	xrpc.RegisterPollGetter(t.TypeName, func() {
		zlog.Info("zobj.Table.Get", typeName)
		t.GetAndUpdateFunc()
	})
	// t.GetAndUpdateFunc()
}

func (t *TableOwner[S]) CallGet() error {
	var slice []S
	err := CallSelect(&slice, zusers.CurrentUser.UserID, t.Where)
	zlog.Info("zobj.Table.CallGet", t.TypeName, err, len(slice))
	if err != nil {
		return err
	}
	// zlog.Info("Got:", t.TypeName, t.Table, len(slice))

	if t.Table != nil {
		t.Table.UpdateSlice(slice, true)
	} else {
		t.Slice = slice
	}
	return nil
}

func (t *TableOwner[S]) CreateTable(getIfUngot bool) *zslicegrid.TableView[S] {
	t.Table = NewTable(nil, &t.Slice, t.TableOptions, t.Where)
	// zlog.Info("CreateTable:", zlog.Pointer(t), t.TypeName, getIfUngot, t.gotten, t.GetAndUpdateFunc != nil)
	if getIfUngot && !t.gotten {
		t.GetAndUpdateFunc()
	}
	return t.Table
}

func NewTable[S any](v *zslicegrid.TableView[S], s *[]S, options zslicegrid.OptionType, where string) *zslicegrid.TableView[S] {
	if v == nil {
		v = &zslicegrid.TableView[S]{}
	}
	var ss S
	objType := zreflect.MakeTypeNameWithType(ss)
	// options |= zslicegrid.AddShortHelperArea
	v.Init(v, s, "ztable."+string(objType), options)
	v.StoreChangedItemsFunc = func(items []S) {
		CallUpdate(items, zusers.CurrentUser.UserID, where)
	}
	// v.DeleteItemsFunc = v.deleteItems
	if options&zslicegrid.AddHeader != 0 {
		addActionButton(v)
	}
	return v
}

func addActionButton[S any](v *zslicegrid.TableView[S]) {
	v.ActionMenu.CreateItemsFunc = func() []zmenu.MenuedOItem {
		var items []zmenu.MenuedOItem
		ids := v.Grid.SelectedIDs()
		noItems := v.NameOfXItemsFunc(ids, true)
		if len(ids) > 0 {
			if v.Options&zslicegrid.AllowDelete != 0 {
				idel := zmenu.MenuedSCFuncAction("Delete "+noItems+"…", zkeyboard.KeyBackspace, 0, func() {
					v.HandleDeleteKey(true, nil)
				})
				items = append(items, idel)
			}
			if v.Options&zslicegrid.AllowDuplicate != 0 {
				idup := zmenu.MenuedSCFuncAction("Duplcate "+noItems, 'D', 0, func() {
					doEdit[S](v, ids, true, true, true)
				})
				items = append(items, idup)
			}
			if v.Options&zslicegrid.AllowEdit != 0 {
				iedit := zmenu.MenuedSCFuncAction("Edit "+noItems, ' ', 0, func() {
					doEdit[S](v, ids, false, false, false)
				})
				items = append(items, iedit)
			}
		}
		if v.Options&zslicegrid.AllowNew != 0 {
			inew := zmenu.MenuedSCFuncAction("New "+v.StructName, 'N', 0, func() {
				var s S
				zfields.CallStructInitializer(&s)
				isEditOnNewStruct := true
				selectAfterEditing := true
				title := "Edit " + v.StructName
				v.EditItems([]S{s}, title, isEditOnNewStruct, selectAfterEditing, nil)
			})
			items = append(items, inew)
		}
		return items
	}
}

func doEdit[S any](v *zslicegrid.TableView[S], ids []string, clearPrimary, initStruct, insert bool) {
	var rows []S
	for _, sid := range ids {
		s := v.StructForID(sid)
		if clearPrimary {
			SetIDInStruct(s, 0, "id")
		}
		rows = append(rows, *s)
	}
	editRows(v, rows, insert)
}

func editRows[S any](v *zslicegrid.TableView[S], rows []S, insert bool) {
	zfields.EditStructSlice(&rows, v.EditParameters, "Edit "+v.StructName, zpresent.ModalConfirmAttributes(), func(ok bool) bool {
		if !ok {
			return true
		}
		go func() {
			var err error
			if insert {
				_, err = CallInsert(rows, zusers.CurrentUser.UserID)
			} else {
				err = CallUpdate(rows, zusers.CurrentUser.UserID, "")
			}
			if err != nil {
				zalert.ShowError(err)
			}
		}()
		return true
	})
}

// func (v *ObjTableView[S]) deleteItems(ids []string) {
// 	var affected int64
// 	if v.Owner.IsQuoteIDs {
// 		for i := range ids {
// 			ids[i] = zsql.QuoteString(ids[i])
// 		}
// 	}
// 	xrpc.MainCaller().Call(v.Owner.rpcCallerName+".PreDeleteRows", ids, nil)

// 	query := "DELETE FROM " + v.Owner.TableName + " WHERE id IN (" + strings.Join(ids, ",") + ")"
// 	err := xrpc.MainCaller().Call("SQLCalls.ExecuteQuery", query, &affected)
// 	if err != nil {
// 		zalert.ShowError(err, "updating")
// 	}
// 	v.RemoveItemsFromSlice(ids)
// 	v.UpdateViewFunc(true, false)
// }
