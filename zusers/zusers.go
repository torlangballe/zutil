package zusers

import (
	"errors"
	"fmt"

	// "github.com/gomodule/redigo/redis"
	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	ID           int64
	UserName     string // this is email or username chosen by user
	Salt         string
	PasswordHash string
	Permissions  []string
}

type Authentication struct {
	UserName   string
	Password   string
	IsRegister bool
}

type AuthResult struct {
	Token  string
	UserID int64
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
	UserNamePasswordWrongError = fmt.Errorf("Incorrect username/email: %w", AuthFailedError)
)

func (u *User) IsAdmin() bool {
	return zstr.StringsContain(u.Permissions, AdminPermission)
}

func (u *User) IsSuper() bool {
	return u.ID == 1
}
