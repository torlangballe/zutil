//go:build !js

package zprocess

import (
	"context"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"

	"github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
	"golang.org/x/sys/unix"
)

func RunBashCommand(command string, timeoutSecs float64) (string, error) {
	return RunCommand("/bin/bash", timeoutSecs, []string{"-c", command}...)
}

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

func GetAppProgramPath(appName string) string {
	return "/Applications/" + appName + ".app/Contents/MacOS/" + appName
}

func RunApp(appName string, args ...string) (cmd *exec.Cmd, outPipe, errPipe io.ReadCloser, inPipe io.WriteCloser, err error) {
	path := GetAppProgramPath(appName)
	return MakeCommand(path, true, args...)
}

func MakeCommand(command string, start bool, args ...string) (cmd *exec.Cmd, outPipe, errPipe io.ReadCloser, inPipe io.WriteCloser, err error) {
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
	inPipe, err = cmd.StdinPipe()
	if err != nil {
		err = zlog.Error(err, "connect stdin pipe")
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

// GetPIDsForAppName returns the pid for all processes with executable name *app*.
// *excludeZombies* checks if it has a ps Z state and doesn't add it to list then. This can take quite a long time.
func GetPIDsForAppName(app string, excludeZombies bool) []int64 {
	var pids []int64
	procs, _ := ps.Processes()
	for _, p := range procs {
		if p.Executable() == app {
			if excludeZombies {
				proc, err := process.NewProcess(int32(p.Pid())) // Specify process id of parent
				if err != nil {
					zlog.Error(err, "new proc")
					continue
				}
				status, err := proc.Status()
				if err != nil {
					zlog.Error(err, "get status")
					continue
				}
				if zstr.StringsContain(status, "Z") {
					continue
				}
			}
			pids = append(pids, int64(p.Pid()))
		}
	}
	// zlog.Info("GetPIDsForAppName", app, len(procs), pids, time.Since(start))
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})
	return pids
}

func terminateProcess(p *process.Process, force, children bool) (oerr error) {
	var err error
	// status, err := p.Status()
	// zlog.Info("terminateProcess:", p.Pid, status, err)
	if children {
		kids, _ := p.Children()
		for _, k := range kids {
			err = terminateProcess(k, force, children)
			if err != nil {
				oerr = err
			}
		}
	}
	if force {
		err = p.Kill()
	} else {
		err = p.Terminate() // Kill the parent process
	}
	// zlog.Info("TerminateAppsByName2", force, err)
	if err != nil {
		oerr = zlog.Wrap(err, "kill main process")
	}
	return
}

func TerminateAppsByName(name string, force, children bool) (oerr error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	excludeZombies := false
	pids := GetPIDsForAppName(name, excludeZombies)
	sort.Slice(pids, func(i, j int) bool {
		return pids[i] < pids[j]
	})
	// zlog.Info("TerminateAppsByName1", name, len(pids))
	for _, pid := range pids {
		p, err := process.NewProcess(int32(pid)) // Specify process id of parent
		// zlog.Info("TerminateAppsByName2:", pid, p.Pid, name, err)
		if err != nil {
			oerr = zlog.Wrap(err, "new child process", pid)
			continue
			// it might try and kill child process after parent, and get error here
			//			return zlog.Error(err, "new process", pid)
		}
		err = terminateProcess(p, force, children)
		if err != nil {
			oerr = err
		}
		// zlog.Info("TerminateAppsByName:", pid, name, err)
	}
	return
}

func GetRunningProcessUserName() (string, error) {
	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		return "", zlog.Error(err, "make process")
	}
	name, err := proc.Username()
	if err != nil {
		return "", zlog.Error(err, "get name")
	}
	return name, nil
}

func SetNumberOfOpenFiles(n int) {
	var rlimit unix.Rlimit

	err := unix.Getrlimit(unix.RLIMIT_NOFILE, &rlimit)
	zlog.OnError(err)
	// zlog.Info("RLIMIT:", rlimit.Cur, rlimit.Max)
	if n <= 0 {
		rlimit.Cur = rlimit.Max
	} else if uint64(n) < rlimit.Max {
		rlimit.Cur = uint64(n)
	}
	err = unix.Setrlimit(unix.RLIMIT_NOFILE, &rlimit)
	zlog.OnError(err)
}

// func SetPriority(pid, priority int) error {
// 	err := syscall.Setpriority(syscall.PRIO_PROCESS, pid, priority)
// 	return err
// }

func RepeatLogProcessUse() {
	ztimer.RepeatNow(60, func() bool {
		procs, _ := ps.Processes()
		zlog.Info("##ProcessCount:", len(procs))
		return true
	})
}
