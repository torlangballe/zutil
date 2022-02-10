package zusers

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/gomodule/redigo/redis"
	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	ID           int64
	Email        string
	Salt         string
	PasswordHash string
	Permissions  []string
}

type Authentication struct {
	Email      string
	Password   string
	IsRegister bool
}

type AuthenticationResult struct {
	Token string
	ID    int64
}

const (
	AdminPermission = "admin"
	hashKey         = "gOBgx69Z3k4TtgTDK8VF"
)

var (
	NotAuthenticatedError     = errors.New("not authenticated")
	database                  *sql.DB
	redisPool                 *redis.Pool
	AuthenticationFailedError = errors.New("Authentication Failed")
	EmailPasswordWrongError   = fmt.Errorf("Incorrect user/email: %w", AuthenticationFailedError)
)

func (u *User) IsAdmin() bool {
	return zstr.StringsContain(u.Permissions, AdminPermission)
}

func (u *User) IsSuper() bool {
	return u.ID == 1
}
