//go:build zui

package zusers

import (
	"fmt"

	"github.com/torlangballe/zui/zalert"
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zui/zbutton"
	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zfields"
	"github.com/torlangballe/zui/zkeyboard"
	"github.com/torlangballe/zui/zlabel"
	"github.com/torlangballe/zui/zpresent"
	"github.com/torlangballe/zui/zshape"
	"github.com/torlangballe/zui/ztext"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zui/zwindow"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zguiutil"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwords"
)

const (
	usernameKey = "zusers.AuthUserName"
)

var (
	UserNameIsEmail        bool = true
	CanCancelAuthDialog    bool = false
	CurrentUser            ClientUserInfo
	AuthenticatedFunc      func(err error) bool
	doingAuth              bool
	MinimumPasswordLength  = 5
	AppSpecificPermissions = []string{"root"}
	NoLoginGUI             bool
)

func Init() {
	token, _ := zkeyvalue.DefaultStore.GetString(tokenKey)
	if token != "" {
		zrpc.MainClient.AuthToken = token
	}
	perms := append([]string{AdminPermission}, AppSpecificPermissions...)
	zfields.SetStringBasedEnum("Permissions", perms...)
}

func StartAuth() {
	zrpc.MainClient.HandleAuthenticationFailedFunc = func() {
		checkAndDoAuth()
	}
	ztimer.StartIn(0.1, checkAndDoAuth)
}

func UserNameType() string {
	if UserNameIsEmail {
		return "Email"
	}
	return "User Name"
}

func OpenDialog(doReg, doLogin, canCancel bool, got func()) {
	const columnWith = 120.0
	v1 := zcontainer.StackViewVert("auth")
	v1.SetSpacing(10)
	v1.SetMarginS(zgeo.SizeD(14, 14))
	v1.SetBGColor(zgeo.ColorNewGray(0.9, 1))

	username, _ := zkeyvalue.DefaultStore.GetString(usernameKey)
	style := ztext.Style{KeyboardType: zkeyboard.TypeDefault}
	if UserNameIsEmail {
		style.KeyboardType = zkeyboard.TypeEmailAddress
	}
	style.AutoCapType = zkeyboard.AutoCapNone
	usernameField := ztext.NewView(username, style, 30, 1)
	str := "username must be an ASCII characters"
	if UserNameIsEmail {
		str = "must be a valid email address"
	}
	usernameField.SetToolTip((str))
	usernameField.UpdateSecs = 0

	style = ztext.Style{KeyboardType: zkeyboard.TypePassword}
	passwordField := ztext.NewView("", style, 60, 1)
	str = fmt.Sprintf("password must be minimum %d ascii characters", MinimumPasswordLength)
	passwordField.SetToolTip((str))
	passwordField.UpdateSecs = 0

	register := zbutton.New(zwords.Register())
	register.SetMinWidth(90)
	register.SetUsable(false)

	login := zbutton.New(zwords.Login())
	login.SetMinWidth(90)
	login.MakeReturnKeyDefault()
	login.SetUsable(false)

	validate := func(edited bool) {
		validateFields(usernameField, passwordField, login, register)
	}
	usernameField.SetValueHandler("", validate)
	passwordField.SetValueHandler("", validate)

	_, s1, _, _ := zguiutil.Labelize(usernameField, UserNameType(), columnWith, zgeo.CenterLeft, "")
	v1.Add(s1, zgeo.TopLeft|zgeo.HorExpand)

	_, s2, _, _ := zguiutil.Labelize(passwordField, "Password", columnWith, zgeo.CenterLeft, "")
	v1.Add(s2, zgeo.TopLeft|zgeo.HorExpand)

	if UserNameIsEmail && doLogin {
		forgot := zlabel.New("Forgot Password?")
		forgot.SetFont(zgeo.FontNice(zgeo.FontDefaultSize-3, zgeo.FontStyleBold))
		forgot.SetColor(zgeo.ColorNavy)
		v1.Add(forgot, zgeo.TopRight)

		forgot.SetPressedHandler("", zkeyboard.ModifierNone, func() {
			doForgot(usernameField.Text())
		})
	}
	h1 := zcontainer.StackViewHor("buttons")
	v1.Add(h1, zgeo.TopLeft|zgeo.HorExpand, zgeo.SizeD(0, 14))

	if doReg {
		h1.Add(register, zgeo.CenterRight)
		register.SetPressedHandler("", zkeyboard.ModifierNone, func() {
			var a Authentication

			a.IsRegister = true
			a.UserName = usernameField.Text()
			a.Password = passwordField.Text()
			go callAuthenticate(v1, a, got)
		})
	}
	if doLogin {
		h1.Add(login, zgeo.CenterRight)
	}
	login.SetPressedHandler("", zkeyboard.ModifierNone, func() {
		var a Authentication

		a.IsRegister = false
		a.UserName = usernameField.Text()
		a.Password = passwordField.Text()
		go callAuthenticate(v1, a, got)
	})
	if canCancel {
		cancel := zshape.ImageButtonViewSimpleInsets("Cancel", "")
		h1.Add(cancel, zgeo.CenterLeft)
		cancel.SetPressedHandler("", zkeyboard.ModifierNone, func() {
			zpresent.Close(v1, true, nil)
		})
	}
	att := zpresent.AttributesNew()
	att.Modal = true
	zpresent.PresentView(v1, att)
}

func doForgot(username string) {
	zalert.PromptForText("Send reset email to address:", username, func(email string) {
		if !zstr.IsValidEmail(email) {
			zalert.Show("Enter a valid email address to set password reset instructions to.")
			return
		}
		err := zrpc.MainClient.Call("UsersCalls.SendResetPasswordMail", email, nil)
		// zlog.Info("Calling:", err)
		if err != nil {
			zalert.ShowError(err)
			return
		}
		zkeyvalue.DefaultStore.SetString(email, usernameKey, true)
		zalert.Show("Reset email sent to:\n", email, "\n\nCheck your inbox and spam mailbox in a little while.")
	})
}

func validateFields(user, pass *ztext.TextView, login, register *zbutton.Button) {
	usable := true
	text := user.Text()
	if UserNameIsEmail {
		if !zstr.IsValidEmail(text) {
			usable = false
		}
	} else {
		if !zstr.IsTypeableASCII(text) {
			usable = false
		}
	}
	text = pass.Text()
	if !zstr.IsTypeableASCII(text) {
		usable = false
	}
	if len(text) < MinimumPasswordLength {
		usable = false
	}
	login.SetUsable(usable)
	register.SetUsable(usable)
}

func callAuthenticate(view zview.View, a Authentication, got func()) {
	var aret ClientUserInfo
	zkeyvalue.DefaultStore.SetString(a.UserName, usernameKey, true)

	err := zrpc.MainClient.Call("UsersCalls.Authenticate", a, &aret)
	if err != nil {
		zalert.ShowError(err)
		return
	}
	if !(a.IsRegister && !AllowRegistration) {
		CurrentUser = aret
		zrpc.MainClient.AuthToken = CurrentUser.Token
		StoreTokenInKeyValueStore(CurrentUser.Token)
	}
	doingAuth = false
	zpresent.Close(view, false, func(dismissed bool) {
		if !dismissed && got != nil {
			got()
		}
	})
}

func checkAndDoAuth() {
	var err error
	// zlog.Info("checkAndDoAuth", zrpc.MainClient.AuthToken)
	if doingAuth {
		return
	}
	doingAuth = true
	var user User
	err = zrpc.MainClient.Call("UsersCalls.GetUserForToken", zrpc.MainClient.AuthToken, &user)
	// zlog.Info("checkAndDoAuth0:", zrpc.MainClient.AuthToken, err)
	if err == nil {
		CurrentUser.UserID = user.ID
		CurrentUser.UserName = user.UserName
		CurrentUser.Permissions = user.Permissions
		CurrentUser.Token = zrpc.MainClient.AuthToken
		// zlog.Info("GetUserForToken:", CurrentUser.UserID)
		if AuthenticatedFunc != nil {
			AuthenticatedFunc(nil)
		}
		doingAuth = false
		return
	}
	// if zrpc.MainClient.AuthToken == "" {
	// 	return
	// }
	if AuthenticatedFunc != nil {
		if AuthenticatedFunc(err) {
			return
		}
	}
	if !NoLoginGUI {
		a := zalert.New(err.Error())
		a.ShowOK(func() {
			showOpenDialog()
		})
	}
}

func showOpenDialog() {
	zlog.Info("open dialog", zpresent.FirstPresented)
	ztimer.RepeatNow(0.1, func() bool {
		if !zpresent.FirstPresented {
			return true
		}
		OpenDialog(AllowRegistration, true, CanCancelAuthDialog, func() {
			if AuthenticatedFunc != nil {
				AuthenticatedFunc(nil)
			}
		})
		return false
	})
}

func showDialogForTextEdit(isPassword, isEmail bool, name, oldValue, title string, got func(newText string)) {
	const columnWith = 120.0
	v1 := zcontainer.StackViewVert("dialog")

	style := ztext.Style{KeyboardType: zkeyboard.TypeDefault}
	if isPassword {
		style.KeyboardType = zkeyboard.TypePassword
	} else if isEmail {
		style.KeyboardType = zkeyboard.TypeEmailAddress
	}
	textField := ztext.NewView(oldValue, style, 60, 1)
	_, s1, _, _ := zguiutil.Labelize(textField, name, columnWith, zgeo.CenterLeft, "")
	v1.Add(s1, zgeo.TopLeft|zgeo.HorExpand)

	att := zpresent.AttributesNew()
	att.Modal = true

	zalert.PresentOKCanceledView(v1, title, att, nil, func(ok bool) bool {
		if ok {
			got(textField.Text())
		}
		return true
	})
}

func HandleResetPassword(args map[string]string) {
	var reset ResetPassword
	reset.ResetToken, _ = args["reset"]
	if reset.ResetToken == "" {
		return
	}
	// zlog.Info("HandleResetPassword", reset.ResetToken)
	stack := zcontainer.StackViewHor("stack")
	att := zpresent.AttributesNew()
	att.MakeFull = true
	att.PresentedFunc = func(win *zwindow.Window) {
		if win == nil {
			return
		}
		// zlog.Info("HandleResetPassword2")
		var resetDialog struct {
			NewPassword string `zui:"password"`
		}
		email := args["email"]
		title := fmt.Sprint("Set new password for user ", email)
		params := zfields.DefaultFieldViewParameters
		params.Field.Flags |= zfields.FlagIsLabelize
		zfields.EditStruct(&resetDialog, params, title, zpresent.AttributesNew(), func(ok bool) bool {
			if !ok {
				return true
			}
			reset.Password = resetDialog.NewPassword
			// zlog.Info("OPASS:", resetDialog.NewPassword)
			go callResetPassword(reset)
			return true
		})
	}
	zpresent.PresentView(stack, att)
	select {}
}

func callResetPassword(reset ResetPassword) {
	var token string
	err := zrpc.MainClient.Call("UsersCalls.SetNewPasswordFromReset", reset, &token)
	if err != nil {
		zalert.ShowError(err)
		return
	}
	StoreTokenInKeyValueStore(token)
	u := zapp.URL()
	q := u.Query()
	q.Del("reset")
	q.Del("email")
	u.RawQuery = q.Encode()
	zlog.Info("GOTOURL:", u.String())
	zwindow.GetMain().SetLocation(u.String())
}
