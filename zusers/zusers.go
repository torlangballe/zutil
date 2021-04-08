package zusers

import (
	"errors"

	"github.com/torlangballe/zutil/zstr"
)

type User struct {
	Id           int64
	Email        string
	Salt         string
	PasswordHash string
	Permissions  []string
}

const (
	AdminPermission = "admin"
	AdminSuperUser  = "super"
	hashKey         = "gOBgx69Z3k4TtgTDK8VF"
)

var NotAuthenticatedError = errors.New("not authenticated")

func (u *User) IsAdmin() bool {
	return zstr.StringsContain(u.Permissions, AdminPermission)
}

func (u *User) IsSuper() bool {
	return zstr.StringsContain(u.Permissions, AdminSuperUser)
}
