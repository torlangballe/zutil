package zusers

import (
	"errors"
	"fmt"
	"time"

	// "github.com/gomodule/redigo/redis"
	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	ID           int64
	UserName     string // this is email or username chosen by user
	Salt         string
	PasswordHash string
	Permissions  []string
	Created      time.Time
	Login        time.Time
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

const (
	AdminPermission = "admin"
)

var (
	AllowRegistration     bool = true
	NotAuthenticatedError      = errors.New("not authenticated")
	// redisPool                 *redis.Pool
	AuthFailedError            = errors.New("Authentication Failed")
	UserNamePasswordWrongError = fmt.Errorf("Incorrect username/email or password: %w", AuthFailedError)
)

func IsAdmin(s []string) bool {
	return zstr.StringsContain(s, AdminPermission)
}

func (u *User) IsAdmin() bool {
	return IsAdmin(u.Permissions)
}

func (u *User) IsSuper() bool {
	return u.ID == 1
}
