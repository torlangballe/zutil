//go:build zui

package zusers

import (
	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zimageview"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
)

func NewActionsIcon() *zimageview.ImageView {
	const (
		change = "change"
		logout = "logout"
	)
	actions := zimageview.New(nil, "images/zcore/head-dark.png", zgeo.Size{18, 18})
	actions.DownsampleImages = true
	actionMenu := zmenu.NewMenuedOwner()
	actionMenu.CreateItems = func() []zmenu.MenuedOItem {
		items := []zmenu.MenuedOItem{
			zmenu.MenuedFuncAction("Logout", logoutUser),
			zmenu.MenuedOItemSeparator,
			zmenu.MenuedFuncAction("Change Password…", changePassword),
			zmenu.MenuedFuncAction("Change "+UserNameType()+"…", changeUserName),
		}
		if IsAdmin(CurrentUser.Permissions) {
			items = append(items,
				zmenu.MenuedOItemSeparator,
				zmenu.MenuedFuncAction("Show User List", showUserList),
			)
		}
		return items
	}
	actionMenu.Build(actions, nil)
	return actions
}

func changePassword() {
	title := "Enter a new password for user " + CurrentUser.UserName
	ShowDialogForTextEdit(true, false, "Password", "", title, func(newPassword string) {
		var change ChangeInfo
		change.NewString = newPassword
		change.UserID = CurrentUser.UserID
		err := zrpc2.MainClient.Call("UsersCalls.ChangePassword", change, &CurrentUser.Token)
		if err != nil {
			zalert.ShowError(err)
			return
		}
		zrpc2.MainClient.AuthToken = CurrentUser.Token
	})
}

func logoutUser() {
	go func() {
		err := zrpc2.MainClient.Call("UsersCalls.Logout", CurrentUser.UserName, nil)
		if err != nil {
			zalert.ShowError(err)
			return
		}
	}()
}

func changeUserName() {
	zlog.Info("changeUsername")
	title := "Enter a new " + UserNameType() + " to change from " + CurrentUser.UserName
	ShowDialogForTextEdit(false, UserNameIsEmail, UserNameType(), "", title, func(newUserName string) {
		var change ChangeInfo
		change.NewString = newUserName
		change.UserID = CurrentUser.UserID
		err := zrpc2.MainClient.Call("UsersCalls.ChangeUserName", change, nil)
		if err != nil {
			zalert.ShowError(err)
			return
		}
		CurrentUser.UserName = newUserName
	})
}

func showUserList() {

}
