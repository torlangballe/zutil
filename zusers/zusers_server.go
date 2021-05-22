// +build server

package zusers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/lib/pq"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zredis"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

type UsersCalls zrpc.CallsBase

var (
	Calls                    = new(UsersCalls)
	StoreAuthenticationError = fmt.Errorf("Store authentication failed: %w", AuthenticationFailedError)
	NoTokenError             = fmt.Errorf("no token for user: %w", AuthenticationFailedError)
)

func Init(db *sql.DB, redis *redis.Pool) error {
	database = db
	redisPool = redis
	squery := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		passwordhash TEXT NOT NULL,
		salt TEXT NOT NULL,
		permissions TEXT[] NOT NULL DEFAULT '{}'
	);
	`
	_, err := database.Exec(squery)
	return err
}

func makeHash(str, salt string) string {
	all := str + salt + hashKey
	return zstr.SHA256Hex([]byte(all))
}

func getUsersFromRows(rows *sql.Rows) (us []*User, err error) {
	for rows.Next() {
		var u User
		err = rows.Scan(&u.ID, &u.Email, pq.Array(&u.Permissions))
		if err != nil {
			return
		}
		us = append(us, &u)
	}
	return
}

func GetUserForID(id int64) (u User, err error) {
	squery := "SELECT id, email, permissions FROM users WHERE id=$1 LIMIT 1"
	row := database.QueryRow(squery, id)
	err = row.Scan(&u.ID, &u.Email, pq.Array(&u.Permissions))
	if err != nil {
		return
	}
	return
}

func DeleteUserForID(id int64) (err error) {
	squery := "DELETE FROM users WHERE id=$1"
	_, err = database.Exec(squery, id)
	return
}

func SetAdminForUser(id int64, isAdmin bool) (err error) {
	var perm []string
	squery := "SELECT permissions FROM users WHERE id=$1"
	tx, err := database.Begin()
	if err != nil {
		return
	}
	defer tx.Commit()
	row := tx.QueryRow(squery, id)
	err = row.Scan(pq.Array(&perm))
	if err != nil {
		return
	}
	perm = zstr.RemovedFromSlice(perm, AdminPermission)
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	squery = "UPDATE users SET permissions=$1 WHERE id=$2"
	_, err = tx.Exec(squery, perm, id)
	return
}

func ChangeEmailForUser(id int64, email string) (err error) {
	squery := "UPDATE users SET email=$1 WHERE id=$2"
	_, err = database.Exec(squery, email, id)
	return
}

func ChangePasswordForUser(id int64, password string) (err error) {
	squery := "UPDATE users SET passwordhash=$1, salt=$2, token=$3 WHERE id=$4"
	salt, hash, token := makeSaltyHash(password)
	_, err = database.Exec(squery, hash, salt, token, id)
	return
}

func GetAllUsers() (us []*User, err error) {
	squery := "SELECT id, email FROM users ORDER BY email ASC"
	rows, err := database.Query(squery)
	if err != nil {
		return
	}
	us, err = getUsersFromRows(rows)
	return
}

func getUserFor(field, value string) (user User, err error) {
	squery :=
		fmt.Sprintf(`SELECT id, email, passwordhash, salt, permissions 
		FROM users WHERE %s=$1 LIMIT 1`, field)
	row := database.QueryRow(squery, value)
	err = row.Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Salt, pq.Array(&user.Permissions))
	if err != nil {
		return
	}
	return
}

func GetUserFromCookieInRequest(req *http.Request) (user User, token string, err error) {
	t, _ := req.Cookie("user-token")
	//	zlog.Info("GetUserFromCookieInRequest:", t.Name, t.Value)
	if t == nil {
		err = NoTokenError
		return
	}
	token = t.Value
	if token == "" {
		err = NoTokenError
		return
	}
	user, err = GetUserFromToken(token)
	//	zlog.Info("GetUserFromCookieInRequest:", err, user)
	if err != nil {
		return
	}
	return
}

func GetUserFromZRPCHeader(req *http.Request) (user User, token string, err error) {
	token, err = zrpc.AuthenticateRequest(req)
	if err != nil || token == "" {
		err = NoTokenError
		zlog.Error(err, "auth", token)
		return
	}
	user, err = GetUserFromToken(token)
	if err != nil {
		return
	}
	return
}

func Login(email, password string) (id int64, token string, err error) {
	u, err := getUserFor("email", email)
	if err != nil {
		if err == sql.ErrNoRows {
			err = EmailPasswordWrongError
		}
		return
	}
	hash := makeHash(password, u.Salt)
	if hash != u.PasswordHash {
		zlog.Info("calchash:", hash, password, "salt:", u.Salt, "storedhash:", u.PasswordHash)
		err = EmailPasswordWrongError
		return
	}
	token = zstr.GenerateUUID()
	id = u.ID
	err = setTokenForUserId(token, id)
	if err != nil {
		err = fmt.Errorf("Storage of authentication failed: %w", AuthenticationFailedError)
		zlog.Info(err)
		return
	}
	return
}

func makeSaltyHash(password string) (salt, hash, token string) {
	salt = zstr.GenerateUUID()
	hash = makeHash(password, salt)
	token = zstr.GenerateUUID()
	return
}

func Register(email, password string, isAdmin, makeToken bool) (id int64, token string, err error) {
	_, err = getUserFor("email", email)
	if err == nil {
		err = errors.New("user already exists: " + email)
		return
	}
	perm := []string{}
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	salt, hash, token := makeSaltyHash(password)
	//	zlog.Info("register:", hash, password, "salt:", salt)
	squery := `
INSERT INTO users (email, passwordhash, salt, permissions) VALUES
($1, $2, $3, $4) RETURNING id`
	row := database.QueryRow(squery, email, hash, salt, pq.Array(perm))
	err = row.Scan(&id)
	if err != nil {
		zlog.Info("register error:", err)
		return
	}
	if makeToken {
		err = setTokenForUserId(token, id)
		if err != nil {
			zlog.Info("set token error:", err)
			return
		}
	}
	return
}

func getUserIDFromRedisFromToken(token string) (id int64, err error) {
	key := "user." + token
	_, err = zredis.Get(redisPool, &id, key)
	if err != nil {
		return 0, AuthenticationFailedError
	}
	return
}

func setTokenForUserId(token string, id int64) (err error) {
	key := "user." + token
	err = zredis.Put(redisPool, key, ztime.Day*30, id)
	return
}

func GetUserFromToken(token string) (user User, err error) {
	id, err := getUserIDFromRedisFromToken(token)
	if err != nil {
		return
	}
	if id == 0 {
		err = fmt.Errorf("no user for token: %w", AuthenticationFailedError)
		return
	}
	return GetUserForID(id)
}

func CheckUserIDInRequest(req *http.Request, id *int64, u *User) error {
	if req == nil {
		zlog.Assert(*id != 0)
		return nil
	}
	user, _, err := GetUserFromZRPCHeader(req)
	if err != nil {
		return err
	}
	if id != nil {
		*id = user.ID
	}
	if u != nil {
		*u = user
	}
	return nil
}

func (u *UsersCalls) CheckIfUserLoggedInWithZRPCHeaderToken(req *http.Request, arg *zrpc.Any, user *User) error {
	return CheckUserIDInRequest(req, nil, user)
}

func (u *UsersCalls) Authenticate(req *http.Request, a *Authentication, r *AuthenticationResult) (err error) {
	if a.IsRegister {
		r.ID, r.Token, err = Register(a.Email, a.Password, false, true)
	} else {
		r.ID, r.Token, err = Login(a.Email, a.Password)
	}
	if err != nil {
		zlog.Error(err, "authenticate", *a)
		return err
	}
	return
}
