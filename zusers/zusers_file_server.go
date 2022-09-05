//go:build server

package zusers

import (
	"sync"
	"time"

	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrpc2"
	"github.com/torlangballe/zutil/zslice"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type FileServer struct {
	storageFolder string
	mutex         sync.Mutex
	lastID        int64
	users         []User
	sessions      []Session
	TokenDuration time.Duration
}

const (
	ufile = "users.json"
	sfile = "sessions.json"
)

func NewFileServer(storageFolder string) (*FileServer, error) {
	var users []User
	var sessions []Session
	err := zjson.UnmarshalFromFile(&users, storageFolder+ufile, true)
	// zlog.Info("Loaded:", resourceID, err, dataPtr, file)
	if err != nil {
		return nil, zlog.NewError(err, "load")
	}
	err = zjson.UnmarshalFromFile(&sessions, storageFolder+sfile, true)
	// zlog.Info("Loaded sessions:", len(sessions))
	if err != nil {
		return nil, zlog.NewError(err, "load")
	}
	var lastID int64
	for i := range users {
		zint.Maximize64(&lastID, users[i].ID)
	}
	s := &FileServer{}
	s.mutex.Lock()
	s.storageFolder = storageFolder
	s.users = users
	s.sessions = sessions
	s.mutex.Unlock()
	s.storageFolder = storageFolder
	s.lastID = lastID
	s.TokenDuration = ztime.Day * 30
	ztimer.RepeatIn(ztime.DurSeconds(ztime.Day), s.removeOldSessions)
	return s, nil
}

func (s *FileServer) removeOldSessions() bool {
	var changed bool
	s.mutex.Lock()
	var j int
	for i := range s.sessions {
		if time.Since(s.sessions[i].Login) > s.TokenDuration {
			changed = true
			s.sessions[i] = s.sessions[i]
			j++
		}
	}
	s.sessions = s.sessions[:j]
	s.mutex.Unlock()
	if changed {
		s.SaveSessions()
	}
	return true
}

func (s *FileServer) IsTokenValid(token string) bool {
	_, err := s.GetUserIDFromToken(token)
	return err == nil
}

func (s *FileServer) findUserForID(id int64) (*User, int) {
	for i, u := range s.users {
		if u.ID == id {
			return &s.users[i], i
		}
	}
	return nil, -1
}

func (s *FileServer) GetUserForID(id int64) (User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	u, _ := s.findUserForID(id)
	if u == nil {
		return User{}, zlog.NewError("No user for id", id)
	}
	return *u, nil
}

func (s *FileServer) DeleteUserForID(id int64) error {
	s.mutex.Lock()
	s.mutex.Unlock()
	_, i := s.findUserForID(id)
	if i == -1 {
		return zlog.NewError("No user to delete for id", id)
	}
	zslice.RemoveAt(&s.users, i)
	return s.SaveUsers()
}

func (s *FileServer) SetAdminForUser(id int64, isAdmin bool) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	u, i := s.findUserForID(id)
	if i == -1 {
		return zlog.NewError("No user to set admin for id", id)
	}
	perm := u.Permissions
	if isAdmin {
		zstr.AddToSet(&perm, AdminPermission)
	} else {
		zstr.ExtractItemFromStrings(&perm, AdminPermission)
	}
	s.users[i].Permissions = perm
	return s.SaveUsers()
}

func (s *FileServer) ChangeUserNameForUser(id int64, username string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i, u := range s.users {
		if u.ID == id {
			s.users[i].UserName = username
			return s.SaveUsers()
		}
	}
	return zlog.NewError("No user to set username for id", id)
}

func (s *FileServer) ChangePasswordForUser(ci zrpc2.ClientInfo, id int64, password string) error {
	hash, salt, token := makeSaltyHash(password)
	s.mutex.Lock()
	u, i := s.findUserForID(id)
	if u == nil {
		s.mutex.Unlock()
		return zlog.NewError("No user to set username for id", id)
	}
	s.users[i].PasswordHash = hash
	s.users[i].Salt = salt
	err := s.SaveUsers()
	s.mutex.Unlock()
	if err != nil {
		return err
	}
	s.UnauthenticateUser(id)
	var session Session
	session.ClientInfo = ci
	session.UserID = id
	session.Token = token
	err = s.AddNewSession(session)
	if err != nil {
		return err
	}
	return nil
}

func (s *FileServer) GetAllUsers() ([]User, error) {
	s.mutex.Lock()
	us := make([]User, len(s.users), len(s.users))
	copy(us, s.users)
	s.mutex.Unlock()
	return us, nil
}

func (s *FileServer) GetUserForUserName(username string) (User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for _, u := range s.users {
		if u.UserName == username {
			return u, nil
		}
	}
	return User{}, zlog.NewError("No user for username", username)
}

func (s *FileServer) GetUserIDFromToken(token string) (id int64, err error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// zlog.Info("FS GetUserIDFromToken:", token, len(s.sessions))
	for _, session := range s.sessions {
		if session.Token == token {
			return session.UserID, nil
		}
	}
	return 0, AuthFailedError
}

func (s *FileServer) AddNewSession(session Session) error {
	_, err := s.GetUserIDFromToken(session.Token)
	if err == nil {
		return zlog.NewError("Token already exists:", session.Token, session.UserID)
	}
	s.mutex.Lock()
	session.Created = time.Now()
	session.Login = session.Created
	s.sessions = append(s.sessions, session)
	err = s.SaveSessions()
	s.mutex.Unlock()
	return err
}

func (s *FileServer) AddNewUser(username, password, hash, salt string, perm []string) (userID int64, err error) {
	var u User
	u.UserName = username
	u.PasswordHash = hash
	u.Salt = salt
	u.Permissions = perm
	s.lastID++
	u.ID = s.lastID
	s.mutex.Lock()
	s.users = append(s.users, u)
	err = s.SaveUsers()
	s.mutex.Unlock()
	return u.ID, err
}

func (s *FileServer) SaveUsers() error {
	// TODO: this locks during file io, worth copying to seperate slice?
	return zjson.MarshalToFile(s.users, s.storageFolder+ufile)
}

func (s *FileServer) SaveSessions() error {
	// TODO: this locks during file io
	return zjson.MarshalToFile(s.sessions, s.storageFolder+sfile)
}

func (s *FileServer) UnauthenticateUser(id int64) error {
	j := 0
	s.mutex.Lock()
	for i := range s.sessions {
		if s.sessions[i].UserID != id {
			s.sessions[j] = s.sessions[i]
			j++
		}
	}
	s.sessions = s.sessions[:j]
	err := s.SaveSessions()
	s.mutex.Unlock()
	return err
}
