//go:build server

package zusers

import (
	"errors"
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

func initialize(s *SQLServer) {
	MainServer = s
	zrpc2.Register(Calls)
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.Authenticate")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SendResetPasswordMail")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SetNewPasswordFromReset")
}

func makeHash(str, salt string) string {
	hash := zstr.SHA256Hex([]byte(str + salt))
	zlog.Info("MakeHash:", hash, "from:", str, salt)
	return hash
}

func makeSaltyHash(password string) (hash, salt, token string) {
	salt = zstr.GenerateUUID()
	hash = makeHash(password, salt)
	token = zstr.GenerateUUID()
	return
}

func (s *SQLServer) Login(ci zrpc2.ClientInfo, username, password string) (ui ClientUserInfo, err error) {
	//	zlog.Info("Login:", username)
	u, err := s.GetUserForUserName(username)
	if err != nil {
		return
	}
	hash := makeHash(password, u.Salt)
	if hash != u.PasswordHash {
		// zlog.Info("calchash:", hash, password, "salt:", u.Salt, "storedhash:", u.PasswordHash)
		err = UserNamePasswordWrongError
		return
	}

	var session Session
	session.ClientInfo = ci
	session.Token = zstr.GenerateUUID()
	zlog.Info("Login:", "hash:", hash, "salt:", u.Salt, "token:", session.Token)
	session.UserID = u.ID
	err = s.AddNewSession(session)
	if err != nil {
		zlog.Error(err, "login", err)
		err = AuthFailedError
		return
	}
	ui.UserName = u.UserName
	ui.Permissions = u.Permissions
	ui.UserID = u.ID
	ui.Token = session.Token
	return
}

func (s *SQLServer) Register(ci zrpc2.ClientInfo, username, password string, isAdmin, makeToken bool) (id int64, token string, err error) {
	if !AllowRegistration {
		return 0, "", zlog.NewError("Registration not allowed")
	}
	_, err = s.GetUserForUserName(username)
	if err == nil {
		err = errors.New("user already exists: " + username)
		return
	}
	perm := []string{}
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	hash, salt, token := makeSaltyHash(password)
	id, err = s.AddNewUser(username, password, hash, salt, perm)
	if makeToken {
		var session Session
		session.ClientInfo = ci
		session.Token = token
		session.UserID = id
		err = s.AddNewSession(session)
		if err != nil {
			zlog.Info("add new session error:", err)
			return
		}
	}
	return
}

func (*UsersCalls) GetUserFromToken(token string, user *User) error {
	u, err := MainServer.GetUserFromToken(token)
	if err != nil {
		return err
	}
	*user = u
	return nil
}

func (*UsersCalls) Logout(ci zrpc2.ClientInfo, username string, reply *zrpc2.Unused) error {
	return MainServer.UnauthenticateToken(ci.Token)
}

func (s *SQLServer) GetUserFromToken(token string) (user User, err error) {
	id, err := s.GetUserIDFromToken(token)
	if err != nil {
		return
	}
	if id == 0 {
		err = fmt.Errorf("no user for token: %w", AuthFailedError)
		return
	}
	return s.GetUserForID(id)
}

func (*UsersCalls) Authenticate(ci zrpc2.ClientInfo, a Authentication, ui *ClientUserInfo) error {
	var err error
	if a.IsRegister {
		ui.UserID, ui.Token, err = MainServer.Register(ci, a.UserName, a.Password, false, true)
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
	err = uc.ChangePassword(ci, change, token)
	return err
}

func (*UsersCalls) ChangePassword(ci zrpc2.ClientInfo, change ChangeInfo, token *string) error {
	var err error
	*token, err = MainServer.ChangePasswordForUser(ci, change.UserID, change.NewString)
	return err
}

func (*UsersCalls) ChangeUserName(ci zrpc2.ClientInfo, change ChangeInfo) error {
	return MainServer.ChangeUserNameForUser(change.UserID, change.NewString)
}
