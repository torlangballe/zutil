//go:build zui

package zusers

import (
	"fmt"
	"net/url"

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
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zwords"
)

const usernameKey = "zusers.AuthUserName"

var (
	UserNameIsEmail     bool = true
	CanCancelAuthDialog bool = false
	AuthenticatedFunc   func(auth AuthResult)
	doingAuth           bool
)

func Init() {
	zrpc2.MainClient.HandleAuthenticanFailedFunc = func() {
		checkAndDoAuth()
	}
	checkAndDoAuth()
}
func OpenDialog(got func(auth AuthResult)) {
	const column = 120.0
	v1 := zcontainer.StackViewVert("auth")
	v1.SetSpacing(10)
	v1.SetMarginS(zgeo.Size{10, 10})
	v1.SetBGColor(zgeo.ColorNewGray(0.9, 1))
	username, _ := zkeyvalue.DefaultStore.GetString(usernameKey)
	style := ztext.Style{KeyboardType: zkeyboard.TypeDefault}
	userNameLabel := "User Name"
	if UserNameIsEmail {
		style.KeyboardType = zkeyboard.TypeEmailAddress
		userNameLabel = "Email"
	}
	usernameField := ztext.NewView(username, style, 20, 1)
	style = ztext.Style{KeyboardType: zkeyboard.TypePassword}
	passwordField := ztext.NewView("", style, 20, 1)
	register := zbutton.New(zwords.Register())
	register.SetMinWidth(90)
	login := zbutton.New(zwords.Login())
	login.SetMinWidth(90)
	login.MakeEnterDefault()

	_, s1, _ := zlabel.Labelize(usernameField, userNameLabel, column, zgeo.CenterLeft)
	v1.Add(s1, zgeo.TopLeft|zgeo.HorExpand)

	_, s2, _ := zlabel.Labelize(passwordField, "Password", column, zgeo.CenterLeft)
	v1.Add(s2, zgeo.TopLeft|zgeo.HorExpand)

	if UserNameIsEmail {
		forgot := zlabel.New("Forgot Password?")
		forgot.SetFont(zgeo.FontDefault().NewWithSize(float64(zgeo.FontStyleBold)))
		forgot.SetColor(zgeo.ColorBlue)
		v1.Add(forgot, zgeo.TopLeft|zgeo.HorExpand)

		forgot.SetPressedHandler(func() {
			zalert.PromptForText("Send reset email to address:", username, func(email string) {
				err := zrpc2.MainClient.Call("UsersCalls.SendResetPasswordMail", email, nil)
				zlog.Info("Calling:", err)
				if err != nil {
					zalert.ShowError(err)
					return
				}
				zkeyvalue.DefaultStore.SetString(email, usernameKey, true)
				zalert.Show("Reset email sent to:\n", email, "\n\nCheck your inbox and spam mailbox in a little while.")
			})
		})
	}
	h1 := zcontainer.StackViewHor("buttons")
	v1.Add(h1, zgeo.TopLeft|zgeo.HorExpand, zgeo.Size{0, 14})

	if AllowRegistration {
		h1.Add(register, zgeo.CenterRight)
		register.SetPressedHandler(func() {
			var a Authentication

			a.IsRegister = true
			a.UserName = usernameField.Text()
			a.Password = passwordField.Text()
			go callAuthenticate(v1, a, got)
		})
	}
	h1.Add(login, zgeo.CenterRight)

	login.SetPressedHandler(func() {
		var a Authentication

		a.IsRegister = false
		a.UserName = usernameField.Text()
		a.Password = passwordField.Text()
		go callAuthenticate(v1, a, got)
	})
	if CanCancelAuthDialog {
		cancel := zshape.ImageButtonViewNewSimple("Cancel", "")
		h1.Add(cancel, zgeo.CenterLeft)
		cancel.SetPressedHandler(func() {
			zpresent.Close(v1, true, nil)
		})
	}
	att := zpresent.AttributesNew()
	att.Modal = true
	zpresent.PresentView(v1, att, nil, nil)
}

func callAuthenticate(view zview.View, a Authentication, got func(auth AuthResult)) {
	var aret AuthResult
	if UserNameIsEmail {
		if !zstr.IsValidEmail(a.UserName) {
			zalert.Show("Invalid email format:\n", a.UserName)
			return
		}
	} else {
		if !zstr.IsTypableASCII(a.UserName) {
			zalert.Show("Invalid username format:\n", a.UserName)
			return
		}
	}
	zkeyvalue.DefaultStore.SetString(a.UserName, usernameKey, true)

	err := zrpc2.MainClient.Call("UsersCalls.Authenticate", a, &aret)
	if err != nil {
		zalert.ShowError(err)
		return
	}
	zlog.Info("callAuthenticate:", aret)
	doingAuth = false
	zpresent.Close(view, false, func(dismissed bool) {
		if !dismissed {
			got(aret)
		}
	})
}

func checkAndDoAuth() {
	if doingAuth {
		return
	}
	doingAuth = true
	const tokenKey = "zusers.AuthToken"
	var user User

	token, _ := zkeyvalue.DefaultStore.GetString(tokenKey)
	// zlog.Info("checkAndDoAuth:", token)
	if token != "" {
		zrpc2.MainClient.AuthToken = token
		err := zrpc2.MainClient.Call("UsersCalls.GetUserFromToken", token, &user)
		if err == nil {
			var auth AuthResult
			auth.UserID = user.ID
			auth.Token = token
			zrpc2.MainClient.AuthToken = auth.Token
			if AuthenticatedFunc != nil {
				AuthenticatedFunc(auth)
			}
			doingAuth = false
			return
		}
		zalert.ShowError(err)
	}
	OpenDialog(func(auth AuthResult) {
		zkeyvalue.DefaultStore.SetString(auth.Token, tokenKey, true)
		zrpc2.MainClient.AuthToken = auth.Token
		zrpc2.MainClient.AuthToken = auth.Token
		if AuthenticatedFunc != nil {
			AuthenticatedFunc(auth)
		}
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
	zpresent.PresentView(stack, att, func(*zwindow.Window) {
		// zlog.Info("HandleResetPassword2")
		var resetDialog struct {
			NewPassword string `zui:"password"`
		}
		email := args["email"]
		title := fmt.Sprint("Set new password for user ", email)
		params := zfields.FieldViewParametersDefault()
		params.LabelizeWidth = 120
		zfields.PresentOKCancelStruct(&resetDialog, params, title, zpresent.AttributesNew(), func(ok bool) bool {
			if !ok {
				return true
			}
			reset.Password = resetDialog.NewPassword
			// zlog.Info("OPASS:", resetDialog.NewPassword)
			go callResetPassword(reset)
			return true
		})
	}, nil)
	select {}
}

func callResetPassword(reset ResetPassword) {
	err := zrpc2.MainClient.Call("UsersCalls.SetNewPasswordFromReset", reset, nil)
	if err != nil {
		zalert.ShowError(err)
		return
	}
	u, _ := url.Parse(zapp.URL())
	u.Query().Del("reset")
	u.Query().Del("email")
	zlog.Info("GOTOURL:", u.String())
	// zwindow.GetMain().SetLocation(u.String())
}
