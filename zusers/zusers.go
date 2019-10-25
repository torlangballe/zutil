package zusers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/garyburd/redigo/redis"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/torlangballe/zutil/ustr"
	"github.com/torlangballe/zutil/zredis"
	"github.com/torlangballe/zutil/ztime"
)

const (
	AdminPermission = "admin"
	AdminSuperUser  = "super"
)

const hashKey = "gOBgx69Z3k4TtgTDK8VF"

type User struct {
	Id           int64
	Email        string
	Salt         string
	PasswordHash string
	OemId        int64
	Permissions  []string
}

func (u *User) IsAdmin() bool {
	return ustr.StringsContain(u.Permissions, AdminPermission)
}

func (u *User) IsSuper() bool {
	return ustr.StringsContain(u.Permissions, AdminSuperUser)
}

var LoginFail error = errors.New("wrong user email or password")

func InitTable(db *sql.DB) error {
	squery := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email TEXT NOT NULL UNIQUE,
		passwordhash TEXT NOT NULL,
		salt TEXT NOT NULL,
		oemid INT NOT NULL,
		permissions TEXT[] NOT NULL DEFAULT '{}'
	);
	`
	_, err := db.Exec(squery)
	return err
}

// func MakeFromStruct(db *sql.DB, u User) (id int64, err error) {
// 	squery := "INSERT INTO users (email, passwordhash, salt, oemid, xxx) VALUES ($1, $2, $3, $4, $5) RETURNING id"
// 	row := db.QueryRow(squery, u.Email, u.PasswordHash, u.Salt, u.OemId, u.IsAdmin)
// 	err = row.Scan(&id)
// 	if err != nil {
// 		fmt.Println("make user err:", err)
// 		err = errors.Wrap(err, "users.MakeFromStruct Query/Scan")
// 		return
// 	}
// 	return
// }

func makeHash(str, salt string) string {
	return ustr.Sha1Hex(str + salt + hashKey)
}

func getUsersFromRows(db *sql.DB, rows *sql.Rows) (us []*User, err error) {
	for rows.Next() {
		var u User
		err = rows.Scan(&u.Id, &u.Email, &u.OemId, pq.Array(&u.Permissions))
		if err != nil {
			return
		}
		us = append(us, &u)
	}
	return
}

func GetUsersForOem(db *sql.DB, oemId int64) (us []*User, err error) {
	squery := "SELECT id, email, oemid, permissions FROM users WHERE oemid=$1 ORDER BY email ASC"
	rows, err := db.Query(squery, oemId)
	if err != nil {
		return
	}
	us, err = getUsersFromRows(db, rows)
	return
}

func GetUserForId(db *sql.DB, id int64) (u User, err error) {
	squery := "SELECT id, email, oemid, permissions FROM users WHERE id=$1 LIMIT 1"
	row := db.QueryRow(squery, id)
	err = row.Scan(&u.Id, &u.Email, &u.OemId, pq.Array(&u.Permissions))
	if err != nil {
		return
	}
	return
}

func DeleteUserForId(db *sql.DB, id int64) (err error) {
	squery := "DELETE FROM users WHERE id=$1"
	_, err = db.Exec(squery, id)
	return
}

func SetAdminForUser(db *sql.DB, id int64, isAdmin bool) (err error) {
	var perm []string
	squery := "SELECT permissions FROM users WHERE id=$1"
	tx, err := db.Begin()
	if err != nil {
		return
	}
	defer tx.Commit()
	row := tx.QueryRow(squery, id)
	err = row.Scan(pq.Array(&perm))
	if err != nil {
		return
	}
	perm = ustr.RemoveStringFromSlice(perm, AdminPermission)
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	squery = "UPDATE users SET permissions=$1 WHERE id=$2"
	_, err = tx.Exec(squery, perm, id)
	return
}

func ChangeEmailForUser(db *sql.DB, id int64, email string) (err error) {
	squery := "UPDATE users SET email=$1 WHERE id=$2"
	_, err = db.Exec(squery, email, id)
	return
}

func ChangePasswordForUser(db *sql.DB, id int64, password string) (err error) {
	squery := "UPDATE users SET passwordhash=$1, salt=$2, token=$3 WHERE id=$4"
	salt, hash, token := makeSaltyHash(password)
	_, err = db.Exec(squery, hash, salt, token, id)
	return
}

func GetAllUsers(db *sql.DB) (us []*User, err error) {
	squery := "SELECT id, email, oemid FROM users ORDER BY email ASC"
	rows, err := db.Query(squery)
	if err != nil {
		return
	}
	us, err = getUsersFromRows(db, rows)
	return
}

func getUserFor(db *sql.DB, field, value string) (user User, err error) {
	squery :=
		fmt.Sprintf(`SELECT id, email, passwordhash, salt, oemid, permissions 
		FROM users WHERE %s=$1 LIMIT 1`, field)
	row := db.QueryRow(squery, value)
	err = row.Scan(&user.Id, &user.Email, &user.PasswordHash, &user.Salt, &user.OemId, pq.Array(&user.Permissions))
	if err != nil {
		return
	}
	return
}

func GetUserFromTokenInRequest(db *sql.DB, redisPool *redis.Pool, req *http.Request) (user User, token string, err error) {
	t, _ := req.Cookie("token")
	//	fmt.Println("GetUserFromTokenInRequest:", t.Name, t.Value)
	if t == nil {
		err = errors.New("no token")
		return
	}
	token = t.Value
	if token == "" {
		err = errors.New("empty token")
		return
	}
	user, err = GetUserFromToken(db, redisPool, token)
	//	fmt.Println("GetUserFromTokenInRequest2:", err, user)
	if err != nil {
		return
	}
	return
}

func Login(db *sql.DB, redisPool *redis.Pool, email, password string) (id int64, token string, err error) {
	u, err := getUserFor(db, "email", email)
	if err != nil {
		if err == sql.ErrNoRows {
			err = LoginFail
		}
		return
	}
	hash := makeHash(password, u.Salt)
	if hash != u.PasswordHash {
		fmt.Println("calchash:", hash, password, "salt:", u.Salt, "storedhash:", u.PasswordHash)
		err = LoginFail
		return
	}
	token = ustr.GenerateUUID()
	id = u.Id
	err = setTokenForUserId(redisPool, token, id)
	if err != nil {
		fmt.Println("login set token error:", err)
		return
	}
	return
}

func makeSaltyHash(password string) (salt, hash, token string) {
	salt = ustr.GenerateUUID()
	hash = makeHash(password, salt)
	token = ustr.GenerateUUID()
	return
}

func Register(db *sql.DB, redisPool *redis.Pool, email, password string, oemId int64, isAdmin, makeToken bool) (id int64, token string, err error) {
	_, err = getUserFor(db, "email", email)
	if err == nil {
		err = errors.New("user exists: " + email)
		return
	}
	perm := []string{}
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	salt, hash, token := makeSaltyHash(password)
	//	fmt.Println("register:", hash, password, "salt:", salt)
	squery :=
		`INSERT INTO users 
	(email, passwordhash, salt, oemid, permissions) VALUES
	($1, $2, $3, $4, $5) RETURNING id
	`
	row := db.QueryRow(squery, email, hash, salt, oemId, pq.Array(perm))
	err = row.Scan(&id)
	if err != nil {
		fmt.Println("register error:", err)
		return
	}
	if makeToken {
		err = setTokenForUserId(redisPool, token, id)
		if err != nil {
			fmt.Println("set token error:", err)
			return
		}
	}
	return
}

func getUserIdFromToken(redisPool *redis.Pool, token string) (id int64, err error) {
	key := "user." + token
	_, err = zredis.Get(redisPool, &id, key)
	return
}

func setTokenForUserId(redisPool *redis.Pool, token string, id int64) (err error) {
	key := "user." + token
	err = zredis.Put(redisPool, key, ztime.Day*30, id)
	return
}

func GetUserFromHeaderToken(db *sql.DB, redisPool *redis.Pool, req *http.Request) (user User, err error) {
	t, _ := req.Cookie("token")
	if t == nil {
		err = errors.New("no token")
		return
	}
	token := t.Value
	if token == "" {
		err = errors.New("empty token")
		return
	}
	user, err = GetUserFromToken(db, redisPool, token)
	if err != nil {
		return
	}
	return
}

func GetUserFromToken(db *sql.DB, redisPool *redis.Pool, token string) (user User, err error) {
	id, err := getUserIdFromToken(redisPool, token)
	if err != nil {
		return
	}
	if id == 0 {
		err = errors.New("no user for token")
		return
	}
	return GetUserForId(db, id)
}
