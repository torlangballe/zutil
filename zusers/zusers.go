package zusers

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	ID           int64     `zui:"static"`
	UserName     string    `zui:"minwidth:200,noautofill"` // this is email or username chosen by user
	Salt         string    `zui:"-"`
	PasswordHash string    `zui:"-"`
	Permissions  []string  `zui:"minwidth:140,enum:Permissions,format:%n|permission"`
	Created      time.Time `zui:"static"`
	Login        time.Time `zui:"static"`
}

type Authentication struct {
	UserName   string
	Password   string
	IsRegister bool
}

type ClientUserInfo struct {
	Token       string
	UserName    string
	Permissions []string
	UserID      int64
}

type ChangeInfo struct {
	UserID    int64
	NewString string
}

type ResetPassword struct {
	ResetToken string
	Password   string
}

type AllUserInfo struct {
	AdminStar string `zui:"notitle,width:16,justify:center,tip:star for admin user"`
	User
	Sessions int `zui:"static,width:70"`
}

const (
	AdminPermission = "admin" // This is someone who can add/delete users, set permissions
	AdminStar       = "★"
)

var (
	AllowRegistration     bool = true
	NotAuthenticatedError      = errors.New("not authenticated")
	// redisPool                 *redis.Pool
	AuthFailedError            = errors.New("Authentication Failed")
	UserNamePasswordWrongError = fmt.Errorf("Incorrect username/email or password: %w", AuthFailedError)
)

func (u User) GetStrID() string {
	return strconv.FormatInt(u.ID, 10)
}

func (u AllUserInfo) GetStrID() string {
	return strconv.FormatInt(u.ID, 10)
}

func IsAdmin(s []string) bool {
	return zstr.StringsContain(s, AdminPermission)
}

func (u *User) IsAdmin() bool {
	return IsAdmin(u.Permissions)
}

func (u *User) IsSuper() bool {
	return u.ID == 1
}
