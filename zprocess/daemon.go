package zprocess

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zhttp"
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
	BinaryPath            string
	Arguments             []string
	LogPath               string
	AddLogURL             string
	Print                 bool // Print out/err to stdout as well. Only really makes sense if running one app only
	EmailToAddresses      []string
	EmailServer           string
	EmailUserID           string
	EmailPassword         string
	EmailFromAddress      string
	EmailFromName         string
	EmailPort             int
	SendLogWaitSecs       int
	StartTime             time.Time
	RestartModifiedBinary bool

	binaryModifiedTime time.Time
	bufferLock         sync.Mutex
	logBuffer          string
	crashBuffer        string
	postLogTimer       *ztimer.Timer
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
	config.Print = true
	config.RestartModifiedBinary = true
	path := os.Args[0] + "-daemon.json"
	stat, _ := os.Stat(path)
	if stat != nil {
		err := zjson.UnmarshalFromFile(&config, path)
		if err != nil {
			return zlog.Error(err, "unmarshal configs")
		}
	}
	// fmt.Printf("READ DAEMON: %+v\n", config)
	config.BinaryPath = os.Args[0]
	config.Arguments = append(config.Arguments, "-znodaemon")
	if adjustConfig != nil {
		adjustConfig(&config)
	}
	config.Spawn()
	return nil
}

func (c *DaemonConfig) putBuffer() {
	c.bufferLock.Lock()
	if c.logBuffer != "" && ztime.Since(c.StartTime) >= float64(c.SendLogWaitSecs) {
		if len(c.crashBuffer)+len(c.logBuffer) > maxCrashBufferSize {
			c.crashBuffer = c.crashBuffer[:maxCrashBufferSize-len(c.logBuffer)]
		}
		c.crashBuffer += c.logBuffer
		cbuf := c.crashBuffer
		c.logBuffer = ""
		c.bufferLock.Unlock()
		if c.AddLogURL != "" {
			params := zhttp.MakeParameters()
			params.Method = http.MethodPut
			str := zstr.ColorRemover.Replace(cbuf)
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
	c.postLogTimer = nil
}

func (c *DaemonConfig) readFromPipe(pipe io.Reader) {
	reader := bufio.NewReader(pipe)
	for {
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
			c.bufferLock.Lock()
			if len(c.logBuffer) > 2048 {
				c.putBuffer()
			} else {
				if c.postLogTimer == nil {
					c.postLogTimer = ztimer.StartIn(5, func() {
						c.putBuffer()
					})
				}
			}
		}
	}
}

func (c *DaemonConfig) sendCrashEmail() {
	subject := "Bridgetech QTT " + strings.Trim(c.BinaryPath, " ./") + " crash"
	c.bufferLock.Lock()
	str := c.crashBuffer
	c.bufferLock.Unlock()
	zlog.Info("SEND CRASH EMAIL:", subject, len(str))
	c.SendEmail(str, subject)
}

func (c *DaemonConfig) Spawn() error {
	zlog.Info("daemon:", c.BinaryPath)
	for {
		c.binaryModifiedTime = zfile.Modified(c.BinaryPath)
		cmd, outPipe, errPipe, err := StartCommand(c.BinaryPath, false, c.Arguments...)
		if err != nil {
			return zlog.Error(err, "start command", c.BinaryPath, c.Arguments)
		}
		c.StartTime = time.Now()
		if c.RestartModifiedBinary {
			ztimer.RepeatIn(1, func() bool {
				mod := zfile.Modified(c.BinaryPath)
				if mod != c.binaryModifiedTime {
					zlog.Info("Binary", c.BinaryPath, "time modified to", ztime.GetNice(mod, true)+". retarting.")
					time.Sleep(time.Second) // just in case
					kerr := cmd.Process.Kill()
					zlog.OnError(kerr, "process kill")
					return false
				}
				return true
			})
		}
		go c.readFromPipe(outPipe)
		go c.readFromPipe(errPipe)
		str := "zprocess daemon: restarting after error in run"
		err = cmd.Run()
		if err != nil {
			c.logBuffer += str + "\n"
			str += " " + err.Error()
		}
		c.putBuffer()
		c.sendCrashEmail()

		time.Sleep(time.Second) // so we don't go completely nuts if something crashes immediately
		fmt.Println(zstr.EscCyan+"#####", str, "#####"+zstr.EscNoColor)
	}
	return nil
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
