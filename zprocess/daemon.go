// +build !js

package zprocess

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sasha-s/go-deadlock"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zfilelog"
	"github.com/torlangballe/zutil/zhost"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zint"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmail"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

const (
	maxCrashBufferSize = 100 * 1024
)

var noDaemon *bool

func init() {
	noDaemon = flag.Bool("znodaemon", false, "set to not spawn self as daemon.")
}

type DaemonConfig struct {
	BinaryPath            string `json:",omitempty"`
	Arguments             []string
	LogPath               string `json:",omitempty"`
	AddLogURL             string `json:",omitempty"`
	Print                 bool   // Print out/err to stdout as well. Only really makes sense if running one app only
	EmailToAddresses      []string
	EmailServer           string    `json:",omitempty"`
	EmailUserID           string    `json:",omitempty"`
	EmailPassword         string    `json:",omitempty"`
	EmailFromAddress      string    `json:",omitempty"`
	EmailFromName         string    `json:",omitempty"`
	EmailPort             int       `json:",omitempty"`
	SendLogWaitSecs       int       `json:",omitempty"`
	StartTime             time.Time `json:"-"`
	RestartModifiedBinary bool
	infoLock              deadlock.Mutex

	binaryModifiedTime time.Time
	bufferLock         deadlock.Mutex
	logBuffer          string
	crashBuffer        string
	postLogTimerLock   deadlock.Mutex
	postLogTimer       *ztimer.Timer
}

func makeConfigPath() string {
	return os.Args[0] + "-daemon.json"
}

func loadConfig(config *DaemonConfig) error {
	path := makeConfigPath()
	stat, _ := os.Stat(path)
	if stat != nil {
		err := zjson.UnmarshalFromFile(&config, path)
		if err != nil {
			return zlog.Error(err, "unmarshal configs")
		}
	}
	return nil
}

func DaemonizeSelf(adjustConfig func(c *DaemonConfig)) error {
	var config DaemonConfig
	if !flag.Parsed() {
		zlog.Info("zprocess.DaemonSelf required that flags have been parsed. Exiting.")
		os.Exit(-1)
	}
	if *noDaemon {
		return nil
	}
	err := loadConfig(&config)
	if err != nil {
		return err
	}
	config.Print = true
	config.RestartModifiedBinary = true
	// fmt.Printf("READ DAEMON: %+v\n", config)
	config.BinaryPath = os.Args[0]
	config.Arguments = append(config.Arguments, "-znodaemon")
	if adjustConfig != nil {
		adjustConfig(&config)
	}
	config.Spawn()
	return nil
}

func ReplaceArgAndResaveConfig(arg, value string) error {
	var config DaemonConfig
	err := loadConfig(&config)
	if err != nil {
		return err
	}
	found := false
	for i, a := range config.Arguments {
		if a == arg && i < len(config.Arguments)-1 {
			config.Arguments[i+1] = value
			found = true
			break
		}
	}
	if !found {
		config.Arguments = append(config.Arguments, arg, value)
	}
	path := makeConfigPath()
	err = zjson.MarshalToFile(config, path)
	if err != nil {
		return err
	}
	return nil
}

func (c *DaemonConfig) putBuffer() {
	c.bufferLock.Lock()
	if c.logBuffer != "" && ztime.Since(c.StartTime) >= float64(c.SendLogWaitSecs) {
		if len(c.crashBuffer)+len(c.logBuffer) > maxCrashBufferSize {
			c.crashBuffer = c.crashBuffer[:zint.Max(0, maxCrashBufferSize-len(c.logBuffer))]
		}
		c.crashBuffer += c.logBuffer
		cbuf := c.crashBuffer
		c.logBuffer = ""
		c.bufferLock.Unlock()
		str := zstr.ColorRemover.Replace(cbuf)
		if str != "" {
			params := zhttp.MakeParameters()
			params.Method = http.MethodPut
			_, err := zhttp.SendBody(c.AddLogURL, params, []byte(str), nil)
			// zlog.Info("putBuffer:", c.AddLogURL)
			if err != nil {
				zlog.Error(err, "put", c.BinaryPath, c.AddLogURL)
				return
			}
		}
	} else {
		c.bufferLock.Unlock()
	}
	c.postLogTimerLock.Lock()
	c.postLogTimer = nil
	c.postLogTimerLock.Unlock()
}

func (c *DaemonConfig) readFromPipe(pipe io.Reader, quit chan struct{}) {
	reader := bufio.NewReader(pipe)
	for {
		select {
		case <-quit:
			zlog.Info("quit read from pipe")
			return
		default:
			break
		}
		str, err := reader.ReadString('\n')
		// zlog.Info("daemon: read from pipe", str, err, len(c.logBuffer))
		if err == io.EOF {
			return
		}
		if strings.HasPrefix(str, "{nolog}") {
			continue
		}
		if c.Print {
			fmt.Print(str)
		}
		if c.AddLogURL != "" {
			c.bufferLock.Lock()
			c.logBuffer += str
			c.bufferLock.Unlock()
			c.postLogTimerLock.Lock()
			if c.postLogTimer == nil {
				c.postLogTimer = ztimer.StartIn(5, func() {
					c.putBuffer()
				})
			}
			c.postLogTimerLock.Unlock()
		}
		if c.LogPath != "" {
			zfilelog.AddToLogFile(c.LogPath, str)
		}
	}
}

func (c *DaemonConfig) sendCrashEmail() {
	_, ip, _ := zhost.GetCurrentLocalIPAddress()
	subject := "Bridgetech QTT " + strings.Trim(c.BinaryPath, " ./") + " @ " + ip + " crashed"
	// fmt.Println("{nolog}SEND CRASH EMAIL1:", subject)
	c.bufferLock.Lock()
	str := c.crashBuffer
	c.bufferLock.Unlock()
	// fmt.Println("{nolog}SEND CRASH EMAIL:", subject, len(str), "\n")
	c.SendEmail(str, subject)
}

func (c *DaemonConfig) Spawn() error {
	zlog.Info("daemon:", c.BinaryPath)
	for {
		sendCrash := true
		c.infoLock.Lock()
		c.binaryModifiedTime = zfile.Modified(c.BinaryPath)
		c.infoLock.Unlock()
		cmd, outPipe, errPipe, err := StartCommand(c.BinaryPath, false, c.Arguments...)
		if err != nil {
			return zlog.Error(err, "start command", c.BinaryPath, c.Arguments)
		}
		c.StartTime = time.Now()
		var lastMod time.Time
		if c.RestartModifiedBinary {
			ztimer.RepeatIn(3, func() bool {
				mod := zfile.Modified(c.BinaryPath)
				c.infoLock.Lock()
				mtime := c.binaryModifiedTime
				c.infoLock.Unlock()
				if mod != mtime {
					if !lastMod.IsZero() && lastMod == mod {
						zlog.Info(zstr.EscCyan+"#### Binary", c.BinaryPath, "time modified to", ztime.GetNice(mod, true)+". retarting. ####"+zstr.EscNoColor)
						time.Sleep(time.Second) // maybe we need this to flush out print
						c.infoLock.Lock()
						kerr := cmd.Process.Kill()
						sendCrash = false
						c.infoLock.Unlock()
						zlog.OnError(kerr, "process kill")
						return false
					}
					lastMod = mod
				}
				return true
			})
		}
		quitReadChannel := make(chan struct{}, 2)
		go c.readFromPipe(outPipe, quitReadChannel)
		go c.readFromPipe(errPipe, quitReadChannel)
		c.infoLock.Lock()
		cmdCopy := cmd
		c.infoLock.Unlock()
		err = cmdCopy.Run()
		str := "zprocess daemon: restarting after error in run"
		fmt.Println("{nolog}exited:", err)
		quitReadChannel <- struct{}{}
		if c.AddLogURL != "" {
			c.putBuffer()
		}
		if err != nil {
			c.bufferLock.Lock()
			c.logBuffer += str + "\n"
			c.bufferLock.Unlock()
			str += " " + err.Error()
		}
		cmd.Process.Kill()
		c.infoLock.Lock()
		send := sendCrash
		c.infoLock.Unlock()
		if send {
			c.sendCrashEmail()
		}
		c.sendRestartSpecialLog(sendCrash)
		time.Sleep(time.Second * 3) // so we don't go completely nuts if something crashes immediately, also to make listeners flush out or we get error on restart
		fmt.Println(zstr.EscCyan+"#####", str, "#####"+zstr.EscNoColor)
	}
	return nil
}

func (c *DaemonConfig) sendRestartSpecialLog(crashed bool) {
	if c.AddLogURL != "" {
		var body string
		if crashed {
			c.bufferLock.Lock()
			body = c.crashBuffer
			c.bufferLock.Unlock()
		}
		if len(body) != 0 {
			params := zhttp.MakeParameters()
			params.Method = http.MethodPut
			surl := c.AddLogURL + "&stopped=" + zhttp.EscapeURLComponent(time.Now().Format(time.RFC3339))
			zlog.Info("\n\n\nWORKER STOPPED!", surl, "\n\n\n")
			_, err := zhttp.SendBody(surl, params, []byte(body), nil)
			if err != nil {
				zlog.Error(err, "send")
			}
		}
	}
}

func (c *DaemonConfig) SendEmail(message, subject string) {
	if len(c.EmailToAddresses) == 0 {
		return
	}
	var m zmail.Mail
	m.From = zmail.Address{Name: c.EmailFromName, Email: c.EmailFromAddress}
	m.Subject = subject
	m.TextContent = message
	for _, a := range c.EmailToAddresses {
		m.To = append(m.To, zmail.Address{Email: a})
	}
	var a zmail.Authentication
	a.UserID = c.EmailUserID
	a.Password = c.EmailPassword
	a.Server = c.EmailServer
	a.Port = c.EmailPort
	err := m.SendWithSMTP(a)
	if err != nil {
		zlog.Error(err, "send", c.EmailUserID, c.EmailPassword, c.EmailServer)
	}
}
