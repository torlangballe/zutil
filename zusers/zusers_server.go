//go:build server

package zusers

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmail"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zstr"
)

// https://github.com/go-webauthn/webauthn -- for passwordless  Web Authentication public key login
// https://css-tricks.com/passkeys-what-the-heck-and-why/

type UsersCalls zrpc.CallsBase

type Session struct {
	zrpc.ClientInfo
	UserID  int64
	Created time.Time
}

type ForgotPasswordData struct {
	MailAuth    zmail.Authentication
	ProductName string
	URL         string
	From        zmail.Address
}

var (
	Calls                    = new(UsersCalls)
	MainServer               *SQLServer
	resetCache               = zcache.NewWithExpiry(60*10, false) // cache of reset-token:email
	StoreAuthenticationError = fmt.Errorf("Store authentication failed: %w", AuthFailedError)
	NoTokenError             = fmt.Errorf("no token for user: %w", AuthFailedError)
	NoUserError              = fmt.Errorf("no user: %w", AuthFailedError)
	ForgotPassword           = ForgotPasswordData{ProductName: "This service"}

	DefaultEmail    = "user@example.com"
	DefaultPassword = "admin"
)

func setupWithSQLServer(s *SQLServer) {
	MainServer = s
	zrpc.Register(Calls)
	zrpc.SetAuthNotNeededForMethod("UsersCalls.Authenticate")
	zrpc.SetAuthNotNeededForMethod("UsersCalls.SendForgotPasswordPasswordMail")
	zrpc.SetAuthNotNeededForMethod("UsersCalls.SetNewPasswordFromForgotPassword")
}

func makeHash(str, salt string) string {
	hash := zstr.SHA256Hex([]byte(str + salt))
	// zlog.Info("MakeHash:", hash, "from:", str, salt)
	return hash
}

func makeSaltyHash(password string) (hash, salt, token string) {
	salt = zstr.GenerateUUID()
	hash = makeHash(password, salt)
	token = zstr.GenerateUUID()
	return
}

func (*UsersCalls) GetUserForToken(token string, user *User) error {
	if MainServer == nil {
		return nil
	}
	u, err := MainServer.GetUserForToken(token)
	zlog.Info("GetUserForToken:", err, token)
	if err != nil {
		zlog.Error(err, "GetUserForToken", token)
		return err
	}
	*user = u
	return nil
}

func (*UsersCalls) Logout(ci *zrpc.ClientInfo, username string, reply *zrpc.Unused) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.UnauthenticateToken(ci.Token)
}

func (*UsersCalls) Authenticate(ci *zrpc.ClientInfo, a Authentication, ui *ClientUserInfo) error {
	zlog.Info("UC.Authenticate:", ui)
	if MainServer == nil {
		return nil
	}
	var err error
	makeToken := true
	if a.IsRegister {
		if !AllowRegistration {
			fail := true
			if ci.Token != "" {
				u, err := MainServer.GetUserForToken(ci.Token)
				fail = !(err == nil && IsAdmin(u.Permissions))
			}
			if fail {
				return zlog.NewError("registration not allowed")
			}
			makeToken = false
		}
		ui.UserID, ui.Token, err = MainServer.Register(ci, a.UserName, a.Password, makeToken)
		ui.UserName = a.UserName
		ui.Permissions = []string{} // nothing yet, we just registered
	} else {
		ci.Token = "" // clear any old token already stored, so Login generates a new one
		*ui, err = MainServer.Login(ci, a.UserName, a.Password)
		zlog.Info("Login:", ui, err)
	}
	if err != nil {
		zlog.Error(err, "authenticate", a, a.IsRegister)
		return err
	}
	return nil
}

func (*UsersCalls) SendForgotPasswordPasswordMail(email string, r *zrpc.Unused) error {
	if MainServer == nil {
		return nil
	}
	var m zmail.Mail
	random := zstr.GenerateRandomHexBytes(16)
	surl, _ := zhttp.MakeURLWithArgs(ForgotPassword.URL, map[string]string{
		"reset": random,
		"email": email,
	})
	m.From.Name = "Tor Langballe"
	m.AddTo("", email)
	m.Subject = "Reset password for " + ForgotPassword.ProductName
	m.TextContent = "Click here to reset your password:\n\n" + surl
	// err := m.SendWithSMTP(ForgotPassword.MailAuth)
	err := m.Send(ForgotPassword.MailAuth)
	if err == nil {
		zlog.Error(err, "forgot password send error:", m, ForgotPassword.MailAuth)
		resetCache.Put(random, email)
	}
	return err
}

func (uc *UsersCalls) SetNewPasswordFromForgotPassword(ci *zrpc.ClientInfo, reset ResetPassword, token *string) error {
	if MainServer == nil {
		return nil
	}
	var email string
	got := resetCache.Get(&email, reset.ResetToken)
	if !got {
		zlog.Error(nil, "no reset initiated:", reset.ResetToken)
		return zlog.NewError("No reset initiated. Maybe you waited more than 10 minutes.")
	}
	u, err := MainServer.GetUserForUserName(email)
	if err != nil {
		zlog.Error(err, "no user")
		return zlog.NewError("No user for that reset link")
	}
	resetCache.Remove(reset.ResetToken)
	var change ChangeInfo
	change.NewString = reset.Password
	change.UserID = u.ID
	err = uc.ChangePasswordForSelf(ci, change, token)
	return err
}

func (*UsersCalls) ChangePasswordForSelf(ci *zrpc.ClientInfo, change ChangeInfo, token *string) error {
	if MainServer == nil {
		return nil
	}
	var err error
	*token, err = MainServer.ChangePasswordForUser(ci, change.UserID, change.NewString)
	return err
}

func (*UsersCalls) ChangeUserNameForSelf(ci *zrpc.ClientInfo, change ChangeInfo) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.ChangeUserNameForUser(change.UserID, change.NewString)
}

func (s *UsersCalls) GetAllUsers(in *zrpc.Unused, us *[]AllUserInfo) error {
	if MainServer == nil {
		return nil
	}
	var err error
	*us, err = MainServer.GetAllUsers()
	return err
}

func (s *UsersCalls) DeleteUserForID(id int64) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.DeleteUserForID(id)
}

func (*UsersCalls) ChangeUsersUserNameAndPermissions(ci *zrpc.ClientInfo, change ClientUserInfo) error {
	if MainServer == nil {
		return nil
	}
	callingUser, err := MainServer.GetUserForToken(ci.Token)
	if err != nil {
		return zlog.Error(err, "getting admin user")
	}
	if !callingUser.IsAdmin() {
		return zlog.Error(err, "change user name / permissions: must be done my admin user")
	}
	changeUser, err := MainServer.GetUserForID(change.UserID)
	if err != nil {
		return zlog.Error(err, "getting change user")
	}
	err = MainServer.ChangeUsersUserNameAndPermissions(ci, change)
	if err != nil {
		return err
	}
	if !zstr.SlicesHaveSameValues(change.Permissions, changeUser.Permissions) {
		err = MainServer.UnauthenticateUser(change.UserID)
		if err != nil {
			zlog.Error(err, "UnauthenticateUser")
		}
	}
	return nil
}

func (*UsersCalls) UnauthenticateUser(ci *zrpc.ClientInfo, userID int64) error {
	if MainServer == nil {
		return nil
	}
	zlog.Info("US.UnauthenticateUser")
	callingUser, err := MainServer.GetUserForToken(ci.Token)
	if err != nil {
		return zlog.Error(err, "getting admin user")
	}
	if !callingUser.IsAdmin() {
		return zlog.Error(err, "unauthenticating other user: must be done my admin user")
	}
	err = MainServer.UnauthenticateUser(userID)
	if err != nil {
		zlog.Error(err, "UnauthenticateUser")
	}
	return nil
}

func RegisterDefaultAdminUserIfNone() {
	var us []AllUserInfo
	err := Calls.GetAllUsers(nil, &us)
	if err == nil && len(us) == 0 {
		userID, _, _ := MainServer.Register(nil, DefaultEmail, DefaultPassword, false)
		MainServer.SetAdminForUser(userID, true)
	}
}
