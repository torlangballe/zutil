//go:build server

package zusers

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lib/pq"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type SQLServer struct {
	zsql.Base
	UseNoSaltMD5Hash bool
}

func NewSQLServer(db *sql.DB, btype zsql.BaseType, executor *zrpc.Executor) (*SQLServer, error) {
	if db == nil {
		setupWithSQLServer(nil, executor)
		return nil, nil
	}
	s := &SQLServer{}
	s.DB = db
	s.Type = btype
	err := s.setup()
	setupWithSQLServer(s, executor)
	return s, err
}

func (s *SQLServer) customizeQuery(query string) string {
	return zsql.CustomizeQuery(query, s.Type)
}

func (s *SQLServer) setup() error {
	squery := `
	CREATE TABLE IF NOT EXISTS zusers (
		id $PRIMARY-INT-INC,
		username TEXT NOT NULL UNIQUE,
		passwordhash TEXT NOT NULL,
		salt TEXT NOT NULL,
		permissions TEXT[] NOT NULL DEFAULT '{}',
		created timestamp NOT NULL DEFAULT $NOW,
		login timestamp NOT NULL DEFAULT $NOW
	)`
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery)
	if err != nil {
		zlog.Error("create users", squery, err)
		return err
	}

	squery = `
	CREATE TABLE IF NOT EXISTS zuser_sessions (
		token TEXT PRIMARY KEY,
		userid BIGINT,
		clientid TEXT,
		useragent TEXT NOT NULL,
		ipaddress TEXT NOT NULL,
		created timestamp NOT NULL DEFAULT $NOW,
		used timestamp NOT NULL DEFAULT $NOW
	)`
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery)
	if err != nil {
		zlog.Error("create tokens", squery, err)
		return err
	}
	squery = `CREATE INDEX IF NOT EXISTS idx_tokens_ids ON zuser_sessions (token, userid)`
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery)
	// zlog.Info("Createindex:", err)
	if err != nil {
		zlog.Error("create token index", squery, err)
		return err
	}
	ztimer.Repeat(ztime.DurSeconds(time.Hour), func() bool {
		squery := `DELETE FROM zuser_sessions WHERE used < $NOW - INTERVAL '30 days'`
		squery = s.customizeQuery(squery)
		return true
	})
	return nil
}

func (s *SQLServer) GetNewestTokenForUserID(userID int64) (token string, err error) {
	squery := "SELECT token FROM zuser_sessions WHERE userid=$1 ORDER BY used DESC LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, userID)
	err = row.Scan(&token)
	return token, err
}

func (s *SQLServer) GetUserForToken(token string) (user User, err error) {
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

func (s *SQLServer) IsTokenValid(token string, req *http.Request) (bool, int64) {
	var userID int64
	squery := "SELECT userid FROM zuser_sessions WHERE token=$1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, token)
	err := row.Scan(&userID)
	if err == sql.ErrNoRows {
		return false, 0
	}
	return true, userID
}

func (s *SQLServer) GetUserForID(id int64) (User, error) {
	var user User
	squery := "SELECT " + allUserFields + " FROM zusers WHERE id=$1 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, id)
	err := row.Scan(&user.ID, &user.UserName, &user.PasswordHash, &user.Salt, pq.Array(&user.Permissions), &user.Created, &user.Login)
	if err != nil {
		return user, fmt.Errorf("No user for id %d (%w)", id, AuthFailedError)
	}
	return user, nil
}

func (s *SQLServer) GetUserIDFromToken(token string) (id int64, err error) {
	squery := "SELECT userid FROM zuser_sessions WHERE token=$1 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, token)
	err = row.Scan(&id)
	if err != nil {
		// zlog.Error(squery, "token:", token, err, zlog.CallingStackString())
		return 0, AuthFailedError
	}
	squery = "UPDATE zuser_sessions SET used=$NOW WHERE token=$1"
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery, token)
	if err != nil {
		zlog.Error(squery, token, err)
		return 0, AuthFailedError
	}
	return
}

func (s *SQLServer) DeleteUserForID(id int64) error {
	squery := "DELETE FROM zusers WHERE id=$1"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, id)
	if err == nil {
		err = s.UnauthenticateUser(id)
	}
	return err
}

func (s *SQLServer) SetAdminForUser(id int64, isAdmin bool) error {
	var perm []string
	squery := "SELECT permissions FROM zusers WHERE id=$1"
	squery = s.customizeQuery(squery)
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()
	row := tx.QueryRow(squery, id)
	err = row.Scan(pq.Array(&perm))
	if err != nil {
		return err
	}
	perm = zstr.RemovedFromSlice(perm, AdminPermission)
	if isAdmin {
		perm = append(perm, AdminPermission)
	}
	squery = "UPDATE zusers SET permissions=$1 WHERE id=$2"
	squery = s.customizeQuery(squery)
	_, err = tx.Exec(squery, pq.Array(perm), id)
	return err
}

func (s *SQLServer) ChangeUserNameForUser(id int64, username string) error {
	squery := "UPDATE zusers SET username=$1 WHERE id=$2"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, username, id)
	return err
}

func (s *SQLServer) ChangePasswordForUser(ci *zrpc.ClientInfo, id int64, password string) (token string, err error) {
	var salt, hash string

	squery := "UPDATE zusers SET passwordhash=$1, salt=$2, login=$NOW WHERE id=$3"
	squery = s.customizeQuery(squery)
	hash, salt, token = s.makeSaltyHash(password)
	_, err = s.DB.Exec(squery, hash, salt, id)
	if err == nil {
		zlog.Info("ChangePASS:", hash)
		err = s.UnauthenticateUser(id)
		if err != nil {
			zlog.Error("unauth user", id, err)
		}
		var session Session
		session.ClientInfo = *ci
		session.UserID = id
		session.Token = zstr.Concat(".", ci.Type, token)
		err = s.AddNewSession(session)
		if err != nil {
			return
		}
	}
	return
}

func (s *SQLServer) GetAllUsers() (us []AllUserInfo, err error) {
	squery := "SELECT id, username, permissions, created, login, (SELECT COUNT(*) FROM zuser_sessions us WHERE us.userid=u.id) FROM zusers u ORDER BY username ASC"
	squery = s.customizeQuery(squery)
	rows, err := s.DB.Query(squery)
	if err != nil {
		return
	}
	for rows.Next() {
		var u AllUserInfo
		err = rows.Scan(&u.ID, &u.UserName, pq.Array(&u.Permissions), &u.Created, &u.Login, &u.Sessions)
		if err != nil {
			return
		}
		us = append(us, u)
	}
	return
}

const allUserFields = "id, username, passwordhash, salt, permissions, created, login"

func (s *SQLServer) GetUserForUserName(username string) (user User, err error) {
	squery := "SELECT " + allUserFields + " FROM zusers WHERE username=$1 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, username)
	err = row.Scan(&user.ID, &user.UserName, &user.PasswordHash, &user.Salt, pq.Array(&user.Permissions), &user.Created, &user.Login)
	if err != nil {
		if err == sql.ErrNoRows {
			err = fmt.Errorf("No user for username %s (%w)", username, AuthFailedError)
		}
		return
	}
	return
}

func (s *SQLServer) UnauthenticateToken(token string) error {
	// zlog.Info("Unauth token", token, zlog.CallingStackString())
	squery := "DELETE FROM zuser_sessions WHERE token=$1"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, token)
	return err
}

func (s *SQLServer) UnauthenticateUser(id int64) error {
	// zlog.Info("Unauth user", id, zlog.CallingStackString())
	squery := "DELETE FROM zuser_sessions WHERE userid=$1"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, id)
	return err
}

func (s *SQLServer) AddNewSession(session Session) error {
	squery := `INSERT INTO zuser_sessions (token, userid, clientid, useragent, ipaddress) VALUES ($1, $2, $3, $4, $5)`
	squery = s.customizeQuery(squery)
	zlog.Info("SQL AddNewSession:", zlog.Full(session))
	_, err := s.DB.Exec(squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
	if err != nil {
		zlog.Error("insert", err, squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
		return err
	}
	squery = "UPDATE zusers SET login=$NOW WHERE id=$1"
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery, session.UserID)
	if err != nil {
		zlog.Error("update user", err, squery, session.UserID)
		return err
	}
	return nil
}

func (s *SQLServer) AddNewUser(username, password, hash, salt string, perm []string) (id int64, err error) {
	squery := `INSERT INTO zusers (username, passwordhash, salt, permissions) VALUES ($1, $2, $3, $4) RETURNING id`
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, username, hash, salt, pq.Array(perm))
	err = row.Scan(&id)
	if err != nil {
		zlog.Error("insert error:", err)
		return
	}
	return
}

func (s *SQLServer) Login(ci *zrpc.ClientInfo, username, password string) (ui ClientUserInfo, err error) {
	return s.baseLogin(ci, username, password, false)
}

func (s *SQLServer) LoginWithPrehashedPassword(ci *zrpc.ClientInfo, username, preHashedPassword string) (ui ClientUserInfo, err error) {
	return s.baseLogin(ci, username, preHashedPassword, true)
}

func (s *SQLServer) baseLogin(ci *zrpc.ClientInfo, username, password string, prehashed bool) (ui ClientUserInfo, err error) {
	//	zlog.Info("Login:", username)
	u, err := s.GetUserForUserName(username)
	if err != nil {
		return
	}
	hash := password
	if !prehashed {
		hash = s.makeHash(password, u.Salt)
	}
	if hash != u.PasswordHash {
		// zlog.Info("calchash:", hash, password, "salt:", u.Salt, "storedhash:", u.PasswordHash)
		err = UserNamePasswordWrongError
		return
	}
	var session Session
	session.ClientInfo = *ci
	if session.Token == "" {
		session.Token = zstr.Concat(".", ci.Type, zstr.GenerateUUID())
	}
	// zlog.Info("Login:", "hash:", hash, "salt:", u.Salt, "token:", session.Token)
	session.UserID = u.ID
	err = s.AddNewSession(session)
	if err != nil {
		zlog.Error("login", err)
		err = AuthFailedError
		return
	}
	ui.UserName = u.UserName
	ui.Permissions = u.Permissions
	ui.UserID = u.ID
	ui.Token = session.Token
	return
}

func (s *SQLServer) RegisterUser(ci *zrpc.ClientInfo, username, password string, makeToken bool) (id int64, token string, err error) {
	_, err = s.GetUserForUserName(username)
	if err == nil {
		err = errors.New("user already exists: " + username)
		return
	}
	perm := []string{}
	hash, salt, token := s.makeSaltyHash(password)
	id, err = s.AddNewUser(username, password, hash, salt, perm)
	if makeToken {
		var session Session
		session.ClientInfo = *ci
		session.Token = zstr.Concat(".", ci.Type, token)
		session.UserID = id
		err = s.AddNewSession(session)
		if err != nil {
			zlog.Info("add new session error:", err)
			return
		}
	}
	return
}

func (s *SQLServer) ChangeUsersUserNameAndPermissions(ci *zrpc.ClientInfo, change ClientUserInfo) error {
	squery := "UPDATE zusers SET username=$1, permissions=$2 WHERE id=$3"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, change.UserName, pq.Array(change.Permissions), change.UserID)
	if err != nil {
		return err
	}
	return nil
}

func (s *SQLServer) GetOrCreateSessionForUserIDAndClientID(ci *zrpc.ClientInfo, userID int64, clientID string) (token string, err error) {
	squery := "SELECT token FROM zuser_sessions us WHERE userid=$1 AND clientid=$2 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, userID, clientID)
	err = row.Scan(&token)
	if err == nil {
		return token, err
	}
	var session Session
	session.ClientInfo = *ci
	session.Token = zstr.Concat(".", ci.Type, zstr.GenerateUUID())
	session.ClientID = clientID
	session.UserID = userID
	err = s.AddNewSession(session)
	if err != nil {
		zlog.Error("GetOrCreateSessionForUserID", err)
		return "", AuthFailedError
	}
	return session.Token, nil
}

func (s *SQLServer) makeHash(str, salt string) string {
	if s.UseNoSaltMD5Hash {
		return zstr.MD5Hex([]byte(str))
	}
	hash := zstr.SHA256Hex([]byte(str + salt))
	return hash
}

func (s *SQLServer) makeSaltyHash(password string) (hash, salt, token string) {
	salt = zstr.GenerateUUID()
	hash = s.makeHash(password, salt)
	token = zstr.GenerateUUID()
	return
}
