package zprocess

import (
	"context"
	"io"
	"os/exec"
	"runtime"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

func RunCommand(command string, timeoutSecs float64, args ...string) (string, error) {
	var cmd *exec.Cmd
	var cancel context.CancelFunc
	if runtime.GOOS == "windows" {
		for i, a := range args {
			args[i] = strings.Replace(a, "&", "^&", -1)
		}
	}
	if timeoutSecs != 0 {
		var ctx context.Context
		// zlog.Info("RunCommand with timeout:", timeoutSecs)
		ctx, cancel = context.WithTimeout(context.Background(), ztime.SecondsDur(timeoutSecs))
		cmd = exec.CommandContext(ctx, command, args...)
	} else {
		cmd = exec.Command(command, args...)
	}
	output, err := cmd.CombinedOutput()
	str := string(output) // strings.Replace(, "\n", "", -1)
	// if err != nil {
	// 	var out string
	// 	for _, a := range args {
	// 		out += "'" + a + "' "
	// 	}
	// 	zlog.Error(err, "Run Command err", "'"+command+"'", out, str)
	// }
	if cancel != nil {
		cancel()
	}
	return str, err
}

func getAppProgramPath(appName string) string {
	return "/Applications/" + appName + ".app/Contents/MacOS/" + appName
}

func RunApp(appName string, args ...string) (cmd *exec.Cmd, outPipe, errPipe io.ReadCloser, err error) {
	path := getAppProgramPath(appName)
	return StartCommand(path, true, args...)
}

func StartCommand(command string, start bool, args ...string) (cmd *exec.Cmd, outPipe, errPipe io.ReadCloser, err error) {
	cmd = exec.Command(command, args...)
	outPipe, err = cmd.StdoutPipe()
	if err != nil {
		err = zlog.Error(err, "connect stdout pipe")
		return
	}
	errPipe, err = cmd.StderrPipe()
	if err != nil {
		err = zlog.Error(err, "connect stderr pipe")
		return
	}
	if start {
		err = cmd.Start()
	}
	if err != nil {
		err = zlog.Error(err, "run")
		return
	}
	// zlog.Info("RunApp:", path, cmd.Process.Pid, args)
	return
}

func RunAppleScript(command string, timeoutSecs float64) (string, error) {
	return RunCommand("osascript", timeoutSecs, "-s", "o", "-e", command)
}

func FindParameterAfterFlag(got *string, args []string, flag string) bool {
	for i := range args {
		if args[i] == flag && i < len(args)-1 {
			*got = args[i+1]
			return true
		}
	}
	return false
}
