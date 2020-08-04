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

	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zjson"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

var noDaemon *bool

func init() {
	noDaemon = flag.Bool("znodaemon", false, "set to not spawn self as daemon.")
}

type DaemonConfig struct {
	BinaryPath          string
	Arguments           []string
	LogPath             string
	AddLogURL           string
	Print               bool // Print out/err to stdout as well. Only really makes sense if running one app only
	CrashEmailAddresses []string
	SendLogWaitSecs     int
	StartTime           time.Time

	logBuffer    string
	postLogTimer *ztimer.Timer
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
	path := os.Args[0] + "-daemon.json"
	stat, _ := os.Stat(path)
	if stat != nil {
		err := zjson.UnmarshalFromFile(&config, path)
		if err != nil {
			return zlog.Error(err, "unmarshal configs")
		}
	}
	config.BinaryPath = os.Args[0]
	config.Arguments = append(config.Arguments, "-znodaemon")
	if adjustConfig != nil {
		adjustConfig(&config)
	}
	config.Spawn()
	return nil
}

func (c *DaemonConfig) putBuffer() {
	if c.logBuffer != "" && ztime.Since(c.StartTime) >= float64(c.SendLogWaitSecs) {
		if c.AddLogURL != "" {
			params := zhttp.MakeParameters()
			params.Method = http.MethodPut
			str := zstr.ColorRemover.Replace(c.logBuffer)
			zlog.Assert(str != "")
			_, err := zhttp.SendBody(c.AddLogURL, params, []byte(str), nil)
			// zlog.Info("putBuffer:", c.AddLogURL)
			if err != nil {
				zlog.Error(err, "put", c.BinaryPath, c.AddLogURL)
				return
			}
		}
		c.logBuffer = ""
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
			c.logBuffer += str
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

func (c *DaemonConfig) Spawn() error {
	zlog.Info("daemon:", c.BinaryPath)
	for {
		cmd, outPipe, errPipe, err := StartCommand(c.BinaryPath, false, c.Arguments...)
		if err != nil {
			return zlog.Error(err, "start command", c.BinaryPath, c.Arguments)
		}
		c.StartTime = time.Now()
		go c.readFromPipe(outPipe)
		go c.readFromPipe(errPipe)
		str := "zprocess daemon: restarting after error in run"
		err = cmd.Run()
		if err != nil {
			c.logBuffer += str + "\n"
			str += " " + err.Error()
		}
		c.putBuffer()
		time.Sleep(time.Second) // so we don't go completely nuts if something crashes immediately
		fmt.Println(zstr.EscCyan+"#####", str, "#####"+zstr.EscNoColor)
	}
	return nil
}
