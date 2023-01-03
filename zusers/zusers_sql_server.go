//go:build server

package zusers

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zsql"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type SQLServer struct {
	zsql.Base
}

func NewSQLServer(db *sql.DB, btype zsql.BaseType) (*SQLServer, error) {
	s := &SQLServer{}
	s.DB = db
	s.Type = btype
	err := s.setup()
	setupWithSQLServer(s)
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
		zlog.Error(err, "create users", squery)
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
		zlog.Error(err, "create tokens", squery)
		return err
	}
	squery = `CREATE INDEX IF NOT EXISTS idx_tokens_ids ON zuser_sessions (token, userid)`
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery)
	// zlog.Info("Createindex:", err)
	if err != nil {
		zlog.Error(err, "create token index", squery)
		return err
	}
	ztimer.RepeatIn(ztime.DurSeconds(time.Hour), func() bool {
		squery := `DELETE FROM zuser_sessions WHERE used < $NOW - INTERVAL '30 days'`
		squery = s.customizeQuery(squery)
		return true
	})
	return nil
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

func (s *SQLServer) IsTokenValid(token string) bool {
	var exists bool
	// zlog.Info("IsTokenValid s:", s != nil)
	squery := "SELECT true FROM zuser_sessions WHERE token=$1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, token)
	row.Scan(&exists)
	return exists
}

func (s *SQLServer) GetUserForID(id int64) (User, error) {
	var user User
	squery := "SELECT " + allUserFields + " FROM zusers WHERE id=$1 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, id)
	err := row.Scan(&user.ID, &user.UserName, &user.PasswordHash, &user.Salt, pq.Array(&user.Permissions), &user.Created, &user.Login)
	if err != nil {
		return user, NoUserError
	}
	return user, nil
}

func (s *SQLServer) GetUserIDFromToken(token string) (id int64, err error) {
	squery := "SELECT userid FROM zuser_sessions WHERE token=$1 LIMIT 1"
	squery = s.customizeQuery(squery)
	row := s.DB.QueryRow(squery, token)
	err = row.Scan(&id)
	if err != nil {
		zlog.Error(err, squery, token)
		return 0, AuthFailedError
	}
	squery = "UPDATE zuser_sessions SET used=$NOW WHERE token=$1"
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery, token)
	if err != nil {
		zlog.Error(err, squery, token)
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

func (s *SQLServer) ChangePasswordForUser(ci zrpc2.ClientInfo, id int64, password string) (token string, err error) {
	var salt, hash string

	squery := "UPDATE zusers SET passwordhash=$1, salt=$2, login=$NOW WHERE id=$3"
	squery = s.customizeQuery(squery)
	hash, salt, token = makeSaltyHash(password)
	_, err = s.DB.Exec(squery, hash, salt, id)
	if err == nil {
		zlog.Info("ChangePASS:", hash)
		err = s.UnauthenticateUser(id)
		if err != nil {
			zlog.Error(err, "unauhth user", id)
		}
		var session Session
		session.ClientInfo = ci
		session.UserID = id
		session.Token = token
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
			err = NoUserError
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
	// zlog.Info("SQL AddNewSession:", zlog.Full(session))
	_, err := s.DB.Exec(squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
	if err != nil {
		zlog.Error(err, "insert", squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
		return err
	}
	squery = "UPDATE zusers SET login=$NOW WHERE id=$1"
	squery = s.customizeQuery(squery)
	_, err = s.DB.Exec(squery, session.UserID)
	if err != nil {
		zlog.Error(err, "update user", squery, session.UserID)
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
		zlog.Error(err, "insert error:")
		return
	}
	return
}

func (s *SQLServer) Login(ci zrpc2.ClientInfo, username, password string) (ui ClientUserInfo, err error) {
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
	// zlog.Info("Login:", "hash:", hash, "salt:", u.Salt, "token:", session.Token)
	session.UserID = u.ID
	err = s.AddNewSession(session)
	if err != nil {
		zlog.Error(err, "login", err)
		err = AuthFailedError
		return
	}
	ui.UserName = u.UserName
	ui.Permissions = u.Permissions
	ui.UserID = u.ID
	ui.Token = session.Token
	return
}

func (s *SQLServer) Register(ci zrpc2.ClientInfo, username, password string, makeToken bool) (id int64, token string, err error) {
	_, err = s.GetUserForUserName(username)
	if err == nil {
		err = errors.New("user already exists: " + username)
		return
	}
	perm := []string{}
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

func (s *SQLServer) ChangeUsersUserNameAndPermissions(ci zrpc2.ClientInfo, change ClientUserInfo) error {
	squery := "UPDATE zusers SET username=$1, permissions=$2 WHERE id=$3"
	squery = s.customizeQuery(squery)
	_, err := s.DB.Exec(squery, change.UserName, pq.Array(change.Permissions), change.UserID)
	if err != nil {
		return err
	}
	return nil
}
