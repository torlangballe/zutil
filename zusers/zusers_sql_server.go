//go:build server

package zusers

import (
	"database/sql"

	"github.com/lib/pq"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zstr"
)

type SQLServer struct {
	database *sql.DB
}

func NewSQLServer(db *sql.DB) (server *SQLServer, err error) {
	squery := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		username TEXT NOT NULL UNIQUE,
		passwordhash TEXT NOT NULL,
		salt TEXT NOT NULL,
		permissions TEXT[] NOT NULL DEFAULT '{}'
	)`
	_, err = db.Exec(squery)
	if err != nil {
		zlog.Error(err, "create users", squery)
		return
	}

	squery = `
	CREATE TABLE IF NOT EXISTS user_sessions (
		token TEXT PRIMARY KEY,
		userid BIGINT,
		clientid TEXT,
		useragent TEXT NOT NULL,
		ipaddress TEXT NOT NULL,
		login timestamp NOT NULL DEFAULT NOW(),
		created timestamp NOT NULL DEFAULT NOW()
	);`
	_, err = db.Exec(squery)
	if err != nil {
		zlog.Error(err, "create tokens", squery)
		return
	}
	squery = `CREATE INDEX IF NOT EXISTS idx_tokens_ids ON user_sessions (token, userid)`
	_, err = db.Exec(squery)
	// zlog.Info("Createindex:", err)
	if err != nil {
		zlog.Error(err, "create token index", squery)
		return
	}
	/*
	   	squery = `
	   	CREATE FUNCTION expire_tokens_delete_old_rows() RETURNS trigger
	       LANGUAGE plpgsql
	       AS $$
	   	BEGIN
	     		DELETE FROM user_sessions WHERE time < NOW() - INTERVAL '30 days';
	     		RETURN NEW;
	   	END;
	   	$$;`
	   	_, err = database.Exec(squery)
	   	zlog.Error(err, "add delete func for trigger")

	   	squery = `CREATE TRIGGER expire_tokens_delete_old_rows_trigger
	   		AFTER INSERT ON user_sessions
	   		EXECUTE PROCEDURE expire_tokens_delete_old_rows();`
	   	_, err = database.Exec(squery)
	   	zlog.Error(err, "add trigger")
	*/
	server = &SQLServer{}
	server.database = db
	return
}

func (s *SQLServer) IsTokenValid(token string) bool {
	var exists bool
	squery := "SELECT true FROM user_sessions WHERE token=$1"
	row := s.database.QueryRow(squery, token)
	row.Scan(&exists)
	return exists
}

func (s *SQLServer) GetUserForID(id int64) (User, error) {
	var u User
	squery := "SELECT id, username, permissions FROM users WHERE id=$1 LIMIT 1"
	row := s.database.QueryRow(squery, id)
	err := row.Scan(&u.ID, &u.UserName, pq.Array(&u.Permissions))
	if err != nil {
		return u, NoUserError
	}
	return u, nil
}

func (s *SQLServer) DeleteUserForID(id int64) error {
	squery := "DELETE FROM users WHERE id=$1"
	_, err := s.database.Exec(squery, id)
	return err
}

func (s *SQLServer) SetAdminForUser(id int64, isAdmin bool) error {
	var perm []string
	squery := "SELECT permissions FROM users WHERE id=$1"
	tx, err := s.database.Begin()
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
	squery = "UPDATE users SET permissions=$1 WHERE id=$2"
	_, err = tx.Exec(squery, perm, id)
	return err
}

func (s *SQLServer) ChangeUserNameForUser(id int64, username string) error {
	squery := "UPDATE users SET username=$1 WHERE id=$2"
	_, err := s.database.Exec(squery, username, id)
	return err
}

func (s *SQLServer) ChangePasswordForUser(ci zrpc2.ClientInfo, id int64, password string) error {
	squery := "UPDATE users SET passwordhash=$1, salt=$2 WHERE id=$3"
	salt, hash, token := makeSaltyHash(password)
	_, err := s.database.Exec(squery, hash, salt, id)
	if err == nil {
		var session Session
		session.ClientInfo = ci
		session.UserID = id
		session.Token = token
		err := s.AddNewSession(session)
		if err != nil {
			return err
		}
	}
	return err
}

func (s *SQLServer) GetAllUsers() (us []User, err error) {
	squery := "SELECT id, username FROM users ORDER BY username ASC"
	rows, err := s.database.Query(squery)
	if err != nil {
		return
	}
	for rows.Next() {
		var u User
		err = rows.Scan(&u.ID, &u.UserName, pq.Array(&u.Permissions))
		if err != nil {
			return
		}
		us = append(us, u)
	}
	return
}

func (s *SQLServer) GetUserForUserName(username string) (user User, err error) {
	squery := `SELECT id, username, passwordhash, salt, permissions FROM users WHERE username=$1 LIMIT 1`
	row := s.database.QueryRow(squery, username)
	err = row.Scan(&user.ID, &user.UserName, &user.PasswordHash, &user.Salt, pq.Array(&user.Permissions))
	if err != nil {
		if err == sql.ErrNoRows {
			err = NoUserError
		}
		return
	}
	return
}

func (s *SQLServer) UnauthenticateUser(id int64) error {
	squery := "DELETE FROM user_sessions WHERE userid=$1"
	_, err := s.database.Exec(squery, id)
	return err
}

func (s *SQLServer) GetUserIDFromToken(token string) (id int64, err error) {
	squery := "SELECT userid FROM user_sessions WHERE token=$1 LIMIT 1"
	row := s.database.QueryRow(squery, token)
	err = row.Scan(&id)
	if err != nil {
		zlog.Error(err, squery, token)
		return 0, AuthFailedError
	}
	return
}

func (s *SQLServer) AddNewSession(session Session) error {
	squery :=
		`INSERT INTO user_sessions (token, userid, clientid, useragent, ipaddress) 
	VALUES ($1, $2, $3, $4, $5)`
	// zlog.Info("SQL AddNewSession:", zlog.Full(session))
	_, err := s.database.Exec(squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
	if err != nil {
		zlog.Error(err, "update", squery, session.Token, session.UserID, session.ClientID, session.UserAgent, session.IPAddress)
		return err
	}
	return nil
}

func (s *SQLServer) AddNewUser(username, password, hash, salt string, perm []string) (id int64, err error) {
	squery := `
	INSERT INTO users (username, passwordhash, salt, permissions) VALUES
	($1, $2, $3, $4) RETURNING id`
	row := s.database.QueryRow(squery, username, hash, salt, pq.Array(perm))
	err = row.Scan(&id)
	if err != nil {
		zlog.Error(err, "insert error:")
		return
	}
	return
}
