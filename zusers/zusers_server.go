//go:build server

package zusers

import (
	"errors"
	"fmt"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmail"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zhttp"
)

type UsersCalls zrpc2.CallsBase

type Session struct {
	zrpc2.ClientInfo
	UserID  int64
	Created time.Time
	Login   time.Time
}

type ResetData struct {
	MailAuth    zmail.Authentication
	ProductName string
	URL         string
	From        zmail.Address
}

func InitServer(u UserServer) {
	MainServer = u
	zrpc2.Register(Calls)
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.Authenticate")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SendResetPasswordMail")
	zrpc2.SetMethodAuthNotNeeded("UsersCalls.SetNewPasswordFromReset")
}

var (
	Calls                    = new(UsersCalls)
	MainServer               UserServer
	resetCache               = zcache.New(time.Minute*10, false) // cache of reset-token:email
	StoreAuthenticationError = fmt.Errorf("Store authentication failed: %w", AuthFailedError)
	NoTokenError             = fmt.Errorf("no token for user: %w", AuthFailedError)
	NoUserError              = fmt.Errorf("no user: %w", AuthFailedError)
	Reset                    = ResetData{ProductName: "This service"}
)

type UserServer interface {
	GetUserForID(id int64) (User, error)
	GetUserForUserName(username string) (User, error)
	DeleteUserForID(id int64) error
	SetAdminForUser(id int64, isAdmin bool) error
	ChangeUserNameForUser(id int64, username string) error
	ChangePasswordForUser(ci zrpc2.ClientInfo, id int64, password string) error
	GetAllUsers() ([]User, error)
	AddNewSession(Session) error
	AddNewUser(username, password, hash, salt string, perm []string) (id int64, err error)
	GetUserIDFromToken(token string) (id int64, err error)
	UnauthenticateUser(id int64) error
	IsTokenValid(token string) bool
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

func GetUserFromToken(s UserServer, token string) (user User, err error) {
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

func Login(s UserServer, ci zrpc2.ClientInfo, username, password string) (id int64, token string, err error) {
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
	session.UserID = u.ID
	err = s.AddNewSession(session)
	if err != nil {
		zlog.Error(err, "login", err)
		err = AuthFailedError
		return
	}
	id = u.ID
	token = session.Token
	return
}

func Register(s UserServer, ci zrpc2.ClientInfo, username, password string, isAdmin, makeToken bool) (id int64, token string, err error) {
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

func (uc *UsersCalls) GetUserFromToken(token string, user *User) error {
	u, err := GetUserFromToken(MainServer, token)
	if err != nil {
		return err
	}
	*user = u
	return nil
}

func (u *UsersCalls) Authenticate(ci zrpc2.ClientInfo, a Authentication, r *AuthResult) error {
	var err error
	if a.IsRegister {
		r.UserID, r.Token, err = Register(MainServer, ci, a.UserName, a.Password, false, true)
	} else {
		r.UserID, r.Token, err = Login(MainServer, ci, a.UserName, a.Password)
		// zlog.Info("Login:", r.UserID, r.Token, err)
	}
	if err != nil {
		zlog.Error(err, "authenticate", a, a.IsRegister)
		return err
	}
	return nil
}

func (uc *UsersCalls) SendResetPasswordMail(email string, r *zrpc2.Unused) error {
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

func (uc *UsersCalls) SetNewPasswordFromReset(ci zrpc2.ClientInfo, reset ResetPassword, r *zrpc2.Unused) error {
	var email string
	got := resetCache.Get(&email, reset.ResetToken)
	if !got {
		zlog.Error(nil, "no reset initiated:", reset.ResetToken)
		// resetCache.ForAll(func(key string, value interface{}) bool {
		// 	zlog.Info("RC:", key, value)
		// 	return true
		// })
		return zlog.NewError("No reset initiated. Maybe you waited more than 10 minutes.")
	}
	u, err := MainServer.GetUserForUserName(email)
	if err != nil {
		zlog.Error(err, "find user to set password:", email)
		return err
	}
	err = MainServer.ChangePasswordForUser(ci, u.ID, reset.Password)
	if err != nil {
		zlog.Error(err, "change password")
		return err
	}
	resetCache.Remove(reset.ResetToken)
	return nil
}
