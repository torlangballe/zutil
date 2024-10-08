//go:build server

package zusers

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmail"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zsql"
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
	MainServer               *SQLServer
	resetCache               = zcache.NewWithExpiry(60*10, false) // cache of reset-token:email
	StoreAuthenticationError = fmt.Errorf("Store authentication failed: %w", AuthFailedError)
	NoTokenError             = fmt.Errorf("no token for user: %w", AuthFailedError)
	ForgotPassword           = ForgotPasswordData{ProductName: "This service"}
	temporaryTokens          = zcache.NewExpiringMap[int64, int64](60) // temporaryTokens are tokens that map to a userid, granting that token access as that user for 60 seconds
	authenticator            zrpc.TokenAuthenticator
)

func setupWithSQLServer(s *SQLServer, executor *zrpc.Executor) {
	MainServer = s
	authenticator = executor.Authenticator
	executor.Register(UsersCalls{})
	executor.SetAuthNotNeededForMethod("UsersCalls.GetUserForToken")
	executor.SetAuthNotNeededForMethod("UsersCalls.Authenticate")
	executor.SetAuthNotNeededForMethod("UsersCalls.SendForgotPasswordPasswordMail")
	executor.SetAuthNotNeededForMethod("UsersCalls.SetNewPasswordFromForgotPassword")
	zsql.GetUserIDFromTokenFunc = MainServer.GetUserIDFromToken
}

// func (UsersCalls) IsAuthWorking(ci *zrpc.ClientInfo, arg zrpc.Unused, token *string) error {
// 	*token = ci.Token // if we actually get here at all, we're good
// 	return nil
// }

func (UsersCalls) GetUserForToken(ci *zrpc.ClientInfo, token string, user *User) error {
	if MainServer == nil {
		return nil
	}
	u, err := MainServer.GetUserForToken(token)
	if err != nil {
		valid, uid := authenticator.IsTokenValid("", ci.Request)
		if valid {
			u, err = MainServer.GetUserForID(uid)
			if err != nil {
				return err
			}
			*user = u
			return nil
		}
		return err
	}
	*user = u
	return nil
}

func (UsersCalls) Logout(ci *zrpc.ClientInfo, username string, reply *zrpc.Unused) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.UnauthenticateToken(ci.Token)
}

func (UsersCalls) Authenticate(ci *zrpc.ClientInfo, a Authentication, ui *ClientUserInfo) error {
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
		ui.UserID, ui.Token, err = MainServer.RegisterUser(ci, a.UserName, a.Password, makeToken)
		ui.UserName = a.UserName
		ui.Permissions = []string{} // nothing yet, we just registered
	} else {
		ci.Token = "" // clear any old token already stored, so Login generates a new one
		*ui, err = MainServer.Login(ci, a.UserName, a.Password)
		zlog.Info("Login:", ui, err)
	}
	if err != nil {
		zlog.Error("authenticate", a, a.IsRegister, err)
		return err
	}
	return nil
}

func (UsersCalls) SendForgotPasswordPasswordMail(email string, r *zrpc.Unused) error {
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
		zlog.Error("forgot password send error:", m, ForgotPassword.MailAuth, err)
		resetCache.Put(random, email)
	}
	return err
}

func (UsersCalls) SetNewPasswordFromForgotPassword(ci *zrpc.ClientInfo, reset ResetPassword, token *string) error {
	if MainServer == nil {
		return nil
	}
	var email string
	got := resetCache.Get(&email, reset.ResetToken)
	if !got {
		zlog.Error("no reset initiated:", reset.ResetToken)
		return zlog.NewError("No reset initiated. Maybe you waited more than 10 minutes.")
	}
	u, err := MainServer.GetUserForUserName(email)
	if err != nil {
		zlog.Error("no user", err)
		return zlog.NewError("No user for that reset link")
	}
	resetCache.Remove(reset.ResetToken)
	var change ChangeInfo
	change.NewString = reset.Password
	change.UserID = u.ID
	err = UsersCalls{}.ChangePasswordForSelf(ci, change, token)
	return err
}

func (UsersCalls) ChangePasswordForSelf(ci *zrpc.ClientInfo, change ChangeInfo, token *string) error {
	if MainServer == nil {
		return nil
	}
	var err error
	*token, err = MainServer.ChangePasswordForUser(ci, change.UserID, change.NewString)
	return err
}

func (UsersCalls) ChangeUserNameForSelf(ci *zrpc.ClientInfo, change ChangeInfo) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.ChangeUserNameForUser(change.UserID, change.NewString)
}

func (UsersCalls) GetAllUsers(in *zrpc.Unused, us *[]AllUserInfo) error {
	if MainServer == nil {
		return nil
	}
	var err error
	*us, err = MainServer.GetAllUsers()
	return err
}

func (UsersCalls) DeleteUserForID(id int64) error {
	if MainServer == nil {
		return nil
	}
	return MainServer.DeleteUserForID(id)
}

func (UsersCalls) ChangeUsersUserNameAndPermissions(ci *zrpc.ClientInfo, change ClientUserInfo) error {
	if MainServer == nil {
		return nil
	}
	callingUser, err := MainServer.GetUserForToken(ci.Token)
	if err != nil {
		return zlog.Error("getting admin user", err)
	}
	if !callingUser.IsAdmin() {
		return zlog.Error("change user name / permissions: must be done my admin user", err)
	}
	changeUser, err := MainServer.GetUserForID(change.UserID)
	if err != nil {
		return zlog.Error("getting change user", err)
	}
	err = MainServer.ChangeUsersUserNameAndPermissions(ci, change)
	if err != nil {
		return err
	}
	if !zstr.SlicesHaveSameValues(change.Permissions, changeUser.Permissions) {
		err = MainServer.UnauthenticateUser(change.UserID)
		if err != nil {
			zlog.Error("UnauthenticateUser", err)
		}
	}
	return nil
}

func (UsersCalls) UnauthenticateUser(ci *zrpc.ClientInfo, userID int64) error {
	if MainServer == nil {
		return nil
	}
	zlog.Info("US.UnauthenticateUser")
	callingUser, err := MainServer.GetUserForToken(ci.Token)
	if err != nil {
		return zlog.Error("getting admin user", err)
	}
	if !callingUser.IsAdmin() {
		return zlog.Error("unauthenticating other user: must be done my admin user", err)
	}
	err = MainServer.UnauthenticateUser(userID)
	if err != nil {
		zlog.Error("UnauthenticateUser", err)
	}
	return nil
}

func RegisterDefaultAdminUserIfNone() {
	var us []AllUserInfo
	err := UsersCalls{}.GetAllUsers(nil, &us)
	zlog.Info("RegisterDefaultAdminUserIfNone", err)
	if err == nil && len(us) == 0 && DefaultUserName != "" {
		userID, _, _ := MainServer.RegisterUser(nil, DefaultUserName, DefaultPassword, false)
		MainServer.SetAdminForUser(userID, true)
	}
}

func AddTemporatyToken(userID int64) (token int64) {
	token = rand.Int63()
	temporaryTokens.Set(token, userID)
	return token
}

func PopTemporaryToken(token int64) (userID int64, got bool) {
	userID, got = temporaryTokens.Get(token)
	if got {
		temporaryTokens.Remove(token)
	}
	return userID, got
}
