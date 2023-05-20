//go:build server

package zterm

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/gliderlabs/ssh"
	"github.com/torlangballe/term"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	zrpc "github.com/torlangballe/zutil/zrpc"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/zusers"
)

// https://xtermjs.org for web client?
// https://pkg.go.dev/github.com/gliderlabs/ssh = docs

type hardUser struct {
	password string
	tokens   []string
}

type Terminal struct {
	port               int
	startText          string
	userIDs            map[string]int64
	sessionPublicKeys  map[string]string // maps sessionID to public key, as it is lost
	hardcodedUsers     map[string]hardUser
	PublicKeyStorePath string
	HandleLine         func(line string, ts *Session) bool
	HandleNewSession   func(ts *Session) func(line string, pos int, key rune) (newLine string, newPos int, ok bool)
}

type Session struct {
	session  ssh.Session
	goterm   *term.Terminal
	Terminal *Terminal
	values   map[string]interface{}
}

func (ts *Session) UserID() int64 {
	return ts.Terminal.userIDs[ts.session.User()]
}

func (ts *Session) SetPrompt(str string) {
	// ts.prompt = ts.session.User() + " " + str
	prompt := ts.session.User() + " " + str
	ts.goterm.SetPrompt(prompt)
}

func (ts *Session) GetValue(key string) interface{} {
	return ts.values[key]
}

func (ts *Session) SetValue(key string, val interface{}) {
	ts.values[key] = val
}

func (ts *Session) ContextSessionID() string {
	return ts.session.Context().SessionID()
}

func (ts *Session) Writer() io.Writer {
	return ts.session
}

func (ts *Session) Writeln(parts ...any) {
	fmt.Fprintln(ts.Writer(), parts...)
}

func (ts *Session) Write(parts ...any) {
	fmt.Fprint(ts.Writer(), parts...)
}

func (ts *Session) Writef(format string, parts ...any) {
	str := fmt.Sprintf(format, parts...)
	fmt.Fprint(ts.Writer(), str)
}

func New(startText string) *Terminal {
	t := &Terminal{}
	t.startText = startText
	t.userIDs = map[string]int64{}
	t.sessionPublicKeys = map[string]string{}
	t.hardcodedUsers = map[string]hardUser{}

	return t
}

func (t *Terminal) AddHardcodedUser(user, pass string) {
	t.hardcodedUsers[user] = hardUser{password: pass}
}

func (s *Session) ReadValueLine() (string, error) {
	return s.goterm.ReadValueLine()
}

func (s *Session) runCommands(commands string) {
	coms := strings.Split(commands, ";")
	for _, com := range coms {
		com = strings.TrimSpace(com)
		if !s.Terminal.HandleLine(com, s) {
			return
		}
	}
}

func (t *Terminal) ListenForever(port int) {
	ssh.Handle(func(s ssh.Session) {
		ts := &Session{}
		ts.session = s
		ts.values = map[string]interface{}{}
		ts.Terminal = t
		ts.goterm = term.NewTerminal(s, ts.session.User()+" /> ")
		autoComplete := t.HandleNewSession(ts)
		if len(s.Command()) != 0 {
			com := strings.Join(s.Command(), " ")
			ts.runCommands(com)
			return
		}
		ts.goterm.AutoCompleteCallback = autoComplete
		if t.startText != "" {
			fmt.Fprintln(ts.session, t.startText)
		}
		for {
			line, err := ts.goterm.ReadLine()
			if err != nil {
				return
			}
			if !t.HandleLine(line, ts) {
				return
			}
		}
	})
	var opts []ssh.Option
	if t.PublicKeyStorePath != "" && (len(t.hardcodedUsers) != 0 || zusers.MainServer != nil) {
		publicKeyOpt := ssh.PublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			skey := "ssh:" + zstr.MD5Hex(key.Marshal())
			for _, us := range t.hardcodedUsers {
				if zstr.StringsContain(us.tokens, skey) {
					return true
				}
			}
			if zusers.MainServer == nil {
				return false
			}
			uid, _ := zusers.MainServer.GetUserIDFromToken(skey)
			// zlog.Info("SSH Try public key", ctx.User(), skey, uid)
			if uid != 0 {
				t.userIDs[ctx.User()] = uid
				zlog.Info("🟨SSH Connected user", ctx.User(), uid, "with public key")
				return true
			}
			// store the public key for this session, to save it when we fall back to password auth
			t.sessionPublicKeys[ctx.SessionID()] = skey
			return false // allow all keys, or use ssh.KeysEqual() to compare against known keys
		})
		opts = append(opts, publicKeyOpt)
	}
	loginOpt := ssh.PasswordAuth(func(ctx ssh.Context, pass string) bool {
		zlog.Info("SSH login?", pass, t.hardcodedUsers)
		for user, us := range t.hardcodedUsers {
			if user == ctx.User() && us.password == pass {
				delete(t.sessionPublicKeys, ctx.SessionID())
				return true
			}
		}
		if zusers.MainServer == nil {
			return false
		}
		var ci zrpc.ClientInfo
		ci.Type = "ssh"
		ci.IPAddress = ctx.RemoteAddr().String()
		ci.UserAgent = ctx.ClientVersion()
		ci.Token = t.sessionPublicKeys[ctx.SessionID()]
		userName := ctx.User()
		cui, err := zusers.MainServer.Login(ci, userName, pass) // call login with read-made token to store in session
		if err != nil {
			zlog.Info("Login error:", err)
			return false
		}
		t.userIDs[userName] = cui.UserID
		delete(t.sessionPublicKeys, ctx.SessionID())
		return true
	})
	opts = append(opts, loginOpt)
	file := zfile.ExpandTildeInFilepath("~/.ssh/id_rsa")
	zlog.Info("ssh Hostkeyfile:", file, zfile.Exists(file))
	if zfile.Exists(file) {
		opts = append(opts, ssh.HostKeyFile(file))
	}
	zlog.Info("🟨SSH listening on port", port)
	err := ssh.ListenAndServe(fmt.Sprint(":", port), nil, opts...)
	if err != nil {
		log.Fatal(err)
	}
}
