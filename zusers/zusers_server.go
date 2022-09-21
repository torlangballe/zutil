//go:build server

package zusers

import (
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmail"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"
)

type UsersCalls zrpc2.CallsBase

type Session struct {
	zrpc2.ClientInfo
	UserID  int64
	Created time.Time
}

type ResetData struct {
	MailAuth    zmail.Authentication
	ProductName string
	URL         string
	From        zmail.Address
}

var (
	Calls                    = new(UsersCalls)
	MainServer               *SQLServer
	resetCache               = zcache.New(time.Minute*10, false) // cache of reset-token:email
	StoreAuthenticationError = fmt.Errorf("Store authentication failed: %w", AuthFailedError)
	NoTokenError             = fmt.Errorf("no token for user: %w", AuthFailedError)
	NoUserError              = fmt.Errorf("no user: %w", AuthFailedError)
	Reset                    = ResetData{ProductName: "This service"}
)

func InitServer(s *SQLServer) {
	MainServer = s
	zrpc2.Register(Calls)
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.Authenticate")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SendResetPasswordMail")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SetNewPasswordFromReset")
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
	u, err := MainServer.GetUserForToken(token)
	if err != nil {
		return err
	}
	*user = u
	return nil
}

func (*UsersCalls) Logout(ci zrpc2.ClientInfo, username string) error { // reply *zrpc2.Unused
	return MainServer.UnauthenticateToken(ci.Token)
}

func (*UsersCalls) Authenticate(ci zrpc2.ClientInfo, a Authentication, ui *ClientUserInfo) error {
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
		*ui, err = MainServer.Login(ci, a.UserName, a.Password)
		// zlog.Info("Login:", r.UserID, r.Token, err)
	}
	if err != nil {
		zlog.Error(err, "authenticate", a, a.IsRegister)
		return err
	}
	return nil
}

func (*UsersCalls) SendResetPasswordMail(email string, r *zrpc2.Unused) error {
	var m zmail.Mail
	random := zstr.GenerateRandomHexBytes(20)
	surl, _ := zhttp.MakeURLWithArgs(Reset.URL, map[string]string{
		"reset": random,
		"email": email,
	})
	m.From.Name = "Tor Langballe"
	m.AddTo("", email)
	m.Subject = "Reset password for " + Reset.ProductName
	m.TextContent = "Click here to reset your password:\n\n" + surl
	err := m.SendWithSMTP(Reset.MailAuth)
	if err == nil {
		zlog.Error(err, "forgot password send error:", m, Reset.MailAuth)
		resetCache.Put(random, email)
	}
	return err
}

func (uc *UsersCalls) SetNewPasswordFromReset(ci zrpc2.ClientInfo, reset ResetPassword, token *string) error {
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

func (*UsersCalls) ChangePasswordForSelf(ci zrpc2.ClientInfo, change ChangeInfo, token *string) error {
	var err error
	*token, err = MainServer.ChangePasswordForUser(ci, change.UserID, change.NewString)
	return err
}

func (*UsersCalls) ChangeUserNameForSelf(ci zrpc2.ClientInfo, change ChangeInfo) error {
	return MainServer.ChangeUserNameForUser(change.UserID, change.NewString)
}

func (s *UsersCalls) GetAllUsers(in *zrpc2.Unused, us *[]AllUserInfo) error {
	var err error
	*us, err = MainServer.GetAllUsers()
	return err
}

func (s *UsersCalls) DeleteUserForID(id int64) error {
	return MainServer.DeleteUserForID(id)
}

func (*UsersCalls) ChangeUsersUserNameAndPermissions(ci zrpc2.ClientInfo, change ClientUserInfo) error {
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

func (*UsersCalls) UnauthenticateUser(ci zrpc2.ClientInfo, userID int64) error {
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
