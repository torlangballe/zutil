//go:build zui

package zusers

import (
	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zimageview"
	"github.com/torlangballe/zui/zmenu"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
)

func NewActionsIcon() *zimageview.ImageView {
	const (
		change = "change"
		logout = "logout"
	)
	actions := zimageview.NewWithCachedPath("images/zcore/head-dark.png", zgeo.SizeD(18, 18))
	actions.DownsampleImages = true
	actionMenu := zmenu.NewMenuedOwner()
	actionMenu.CreateItemsFunc = func() []zmenu.MenuedOItem {
		if CurrentUser.UserID == 0 {
			return []zmenu.MenuedOItem{}
		}
		var items []zmenu.MenuedOItem
		isAdmin := IsAdmin(CurrentUser.Permissions)
		if CurrentUser.UserID != 0 {
			user := CurrentUser.UserName
			if isAdmin {
				user += " " + AdminStar
			}
			items = append(items, zmenu.MenuedFuncAction(user, showProfile), zmenu.MenuedOItemSeparator)
		}
		items = append(items,
			zmenu.MenuedFuncAction("Change Password…", changePassword),
			zmenu.MenuedFuncAction("Change "+UserNameType()+"…", changeUserName),
		)
		if isAdmin {
			items = append(items,
				zmenu.MenuedOItemSeparator,
				zmenu.MenuedFuncAction("Show User List", showUserList),
			)
			if !AllowRegistration {
				items = append(items,
					zmenu.MenuedFuncAction("Add User", addUser),
				)
			}
		}
		items = append(items,
			zmenu.MenuedOItemSeparator,
			zmenu.MenuedFuncAction("Logout", logoutUser),
		)
		return items
	}
	actionMenu.Build(actions, nil)
	return actions
}

func changePassword() {
	title := "Enter a new password for user " + CurrentUser.UserName
	showDialogForTextEdit(true, false, "Password", "", title, func(newPassword string) {
		var change ChangeInfo
		change.NewString = newPassword
		change.UserID = CurrentUser.UserID
		err := zrpc.MainClient.Call("UsersCalls.ChangePasswordForSelf", change, &CurrentUser.Token)
		if err != nil {
			zalert.ShowError(err)
			return
		}
		zrpc.MainClient.AuthToken = CurrentUser.Token
	})
}

func logoutUser() {
	go func() {
		err := zrpc.MainClient.Call("UsersCalls.Logout", CurrentUser.UserName, nil)
		if err != nil {
			zalert.ShowError(err)
			return
		}
	}()
}

func changeUserName() {
	title := "Enter a new " + UserNameType() + " to change from " + CurrentUser.UserName
	showDialogForTextEdit(false, UserNameIsEmail, UserNameType(), "", title, func(newUserName string) {
		var change ChangeInfo
		change.NewString = newUserName
		change.UserID = CurrentUser.UserID
		err := zrpc.MainClient.Call("UsersCalls.ChangeUserNameForSelf", change, nil)
		if err != nil {
			zalert.ShowError(err)
			return
		}
		CurrentUser.UserName = newUserName
	})
}

func showUserList() {
	go getAndShowUserList()
}

func addUser() {
	OpenDialog(true, false, true, func() {
		zlog.Info("Regged new user")
	})
}

func showProfile() {

}
