package zusers

import (
	"database/sql"
	"errors"

	"github.com/garyburd/redigo/redis"
	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	ID           int64
	Email        string
	Salt         string
	PasswordHash string
	Permissions  []string
}

const (
	AdminPermission = "admin"
	hashKey         = "gOBgx69Z3k4TtgTDK8VF"
)

var (
	NotAuthenticatedError = errors.New("not authenticated")
	database              *sql.DB
	redisPool             *redis.Pool
	LoginFail             error = errors.New("wrong user email or password")
)

func (u *User) IsAdmin() bool {
	return zstr.StringsContain(u.Permissions, AdminPermission)
}

func (u *User) IsSuper() bool {
	return u.ID == 1
}
