//go:build zui

package zusers

import (
	"strconv"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zslicegrid"
	"github.com/torlangballe/zui/zstyle"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
)

type userTable struct {
	users []AllUserInfo
	grid  *zslicegrid.TableView[AllUserInfo]
}

func (t *userTable) findUser(id string) (*AllUserInfo, int) {
	n, _ := strconv.ParseInt(id, 10, 64)
	for i, u := range t.users {
		if u.ID == n {
			return &t.users[i], i
		}
	}
	return nil, -1
}

func makeTableOwner(users []AllUserInfo) *userTable {
	ut := &userTable{}
	ut.users = users
	ut.grid = zslicegrid.TableViewNew(&ut.users, "users-table", zslicegrid.AddEditDelete|zslicegrid.AddHeader|zslicegrid.AddBar)
	ut.grid.EditParameters.SkipFieldNames = []string{"AdminStar"}
	ut.grid.FieldParameters.AllStatic = true
	ut.grid.FieldParameters.HideStatic = false
	ut.grid.SetBGColor(zstyle.DefaultBGColor())
	ut.grid.Grid.MinRowsForFullSize = 10
	ut.grid.Grid.MaxRowsForFullSize = 10
	ut.grid.Grid.MakeFullSize = true
	ut.grid.StructName = "user"
	baseFunc := ut.grid.NameOfXItemsFunc
	ut.grid.NameOfXItemsFunc = func(ids []string, singleSpecial bool) string {
		if len(ids) == 1 && singleSpecial {
			u, _ := ut.findUser(ids[0])
			return `"` + u.UserName + `"`
		}
		return baseFunc(ids, singleSpecial)
	}
	old := ut.grid.DeleteItemsFunc
	ut.grid.DeleteItemsFunc = func(ids []string) {
		if !ut.checkBeforeDeleteItems(ids) {
			return
		}
		old(ids)
	}
	ut.grid.StoreChangedItemFunc = storeUser
	ut.grid.DeleteItemFunc = deleteUser
	unauthButton := zbutton.New("")
	unauthButton.SetObjectName("unauthorize")
	unauthButton.SetMinWidth(170)
	ut.grid.Bar.Add(unauthButton, zgeo.CenterLeft)
	ut.grid.UpdateWidgets()
	unauthButton.SetPressedHandler(func() {
		go ut.unauthorizeUsers(ut.grid.Grid.SelectedIDs())
	})
	return ut
}

func (t *userTable) checkBeforeDeleteItems(ids []string) bool {
	for _, id := range ids {
		u, _ := t.findUser(id)
		zlog.Info("checkBeforeStoreChangedItems:", u.ID, CurrentUser.UserID)
		if u.ID == CurrentUser.UserID {
			zalert.Show("You can't delete your own user")
			return false
		}
	}
	return true
}

func storeUser(item AllUserInfo, showErr *bool, last bool) error {
	var changed ClientUserInfo
	changed.Permissions = item.Permissions
	changed.UserID = item.ID
	changed.UserName = item.UserName
	err := zrpc2.MainClient.Call("UsersCalls.ChangeUsersUserNameAndPermissions", changed, nil)
	// zlog.Info("ChangeUsersUserNameAndPermissions", changed.Permissions, err)
	if err != nil && *showErr {
		*showErr = false
		zalert.ShowError(err, "update user", item.UserName)
	}
	return err
}

func deleteUser(item *AllUserInfo, showErr *bool, last bool) error {
	err := zrpc2.MainClient.Call("UsersCalls.DeleteUserForID", item.ID, nil)
	zlog.Info("deleteUsers", item.ID, item.UserName, err)
	if err != nil && *showErr {
		*showErr = false
		zalert.ShowError(err, "delete user", item.UserName)
	}
	return err
}

func (t *userTable) unauthorizeUsers(sids []string) {
	var shownError bool
	for _, sid := range sids {
		id, _ := strconv.ParseInt(sid, 10, 64)
		err := zrpc2.MainClient.Call("UsersCalls.UnauthenticateUser", id, nil)
		if err != nil && !shownError {
			zalert.ShowError(err)
			shownError = true
		}
		u, _ := t.findUser(sid)
		u.Sessions = 0
	}
	t.grid.UpdateViewFunc()
}

func (u *AllUserInfo) HandleAction(f *zfields.Field, action zfields.ActionType, view *zview.View) bool {
	// zlog.Info("Action:", action, zfields.ID(f))
	if f == nil {
		return false
	}
	switch action {
	case zfields.SetupFieldAction:
		if f.FieldName == "UserName" && UserNameIsEmail {
			f.Name = "Email Address"
			f.Format = "email"
		}
		return false
	case zfields.DataChangedAction:
		if f.FieldName == "UserName" {
			label, _ := (*view).(*zlabel.Label)
			if label != nil { // it is null if it's a TextView in edit dialog
				font := label.Font()
				if u.UserName == CurrentUser.UserName {
					font = font.NewWithStyle(zgeo.FontStyleBold)
					label.SetFont(font)
				}
			}
		}
		if f.FieldName == "AdminStar" {
			str := ""
			if IsAdmin(u.Permissions) {
				str = AdminStar
			}
			u.AdminStar = str
		}
	}
	return false
}

func getAndShowUserList() {
	var us []AllUserInfo
	err := zrpc2.MainClient.Call("UsersCalls.GetAllUsers", nil, &us)
	if err != nil {
		zalert.ShowError(err)
		return
	}
	// zlog.Info("USERS:", zlog.Full(us))
	table := makeTableOwner(us)
	att := zpresent.AttributesNew()
	att.Modal = true
	att.ModalCloseOnOutsidePress = true
	zpresent.PresentTitledView(table.grid, "Users", att, nil, nil, nil, nil)
}
