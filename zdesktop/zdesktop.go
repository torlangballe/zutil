// +build !js

package zdesktop

import (
	"runtime"
	"sort"

	"github.com/mitchellh/go-ps"
	"github.com/shirou/gopsutil/process"
	"github.com/torlangballe/zutil/zlog"
)

func GetPIDsForAppName(app string) []int64 {
	var pids []int64
	procs, _ := ps.Processes()
	for _, p := range procs {
		// fmt.Println("PROC:", app, "=", p.Executable())
		if p.Executable() == app {
			pids = append(pids, int64(p.Pid()))
		}
	}
	return pids
}

func terminateProcess(p *process.Process, force, children bool) (oerr error) {
	var err error
	if children {
		kids, _ := p.Children()
		for _, k := range kids {
			// zlog.Info("terminateProcess kids:", p.Pid, k.Pid)
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
	// zlog.Info("TerminateAppsByName2", err)
	if err != nil {
		oerr = zlog.Wrap(err, "kill main process")
	}
	return
}

func TerminateAppsByName(name string, force, children bool) (oerr error) {
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	pids := GetPIDsForAppName(name)
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
