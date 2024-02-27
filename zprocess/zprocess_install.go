//go:build !js

package zprocess

import (
	"os"
	"os/user"
	"strings"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

type Installer struct {
	Company     string
	ProductName string
	Domain      string
	UserName    string // username to set owner of file/dirs
}

const launchAgentPlistStr = `
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
	<dict>
		<key>Label</key>
		<string>{id}</string>
		<key>RunAtLoad</key>
		<true/>
		<key>KeepAlive</key>
		<true/>
		<key>SoftResourceLimits</key>
		<dict>
			<key>NumberOfFiles</key>
			<integer>2048</integer>
		</dict>
        <key>StandardOutPath</key>
        <string>{log}</string>
        <key>StandardErrorPath</key>
        <string>{log}</string>
		<key>ProcessType</key>
		<string>Interactive</string>
		<key>Nice</key>
		<integer>-10</integer>
        <key>ProgramArguments</key>
        <array>
			<string>{bin}</string>
			{args}
        </array>
        <key>WorkingDirectory</key>
        <string>{bindir}/</string>
	</dict>
</plist>`

const serviceFileStr = `
[Unit]

[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
PermissionsStartOnly=true
ExecStart={bin} {args}
Restart=always
User={user}
StandardOutput=journal
WorkingDirectory={bindir}
[Install]
WantedBy=default.target
`

func (i *Installer) addPath(isFile bool, parts ...string) string {
	var path string
	for j, part := range parts {
		path = zfile.JoinPathParts(path, part)
		if j != len(parts)-1 || !isFile {
			err := zfile.MakeDirAllIfNotExists(path)
			zlog.AssertNotError(err, path)
		}
		if i.UserName != "" {
			if zfile.Exists(path) {
				err := zfile.SetOwnerAndMainGroup(path, i.UserName)
				zlog.AssertNotError(err, path, i.UserName)
			}
		}
	}
	return path
}

func (i *Installer) OptPath(isFile bool, parts ...string) string {
	str := "/opt/"
	if OnDarwin() {
		str = zfile.ExpandTildeInFilepath("~/opt/")
	}
	dir := zfile.JoinPathParts(str, i.Company, i.ProductName)
	parts = append([]string{dir}, parts...)
	return i.addPath(isFile, parts...)
}

func (i *Installer) BinPath(isFile bool, parts ...string) string {
	parts = append([]string{"bin"}, parts...)
	return i.OptPath(isFile, parts...)
}

func (i *Installer) VarPath(isFile bool, parts ...string) string {
	parts = append([]string{"etc", "var"}, parts...)
	return i.OptPath(isFile, parts...)
}

func (i *Installer) ID() string {
	return zstr.Concat(".", i.Domain, i.Company, i.ProductName)
}

func (i *Installer) InstallProgramWithLauncher(args []string, copyBinary bool) error {
	user, _ := user.Current()
	if user != nil {
		i.UserName = user.Username
	}
	// zlog.Info("InstallProgramWithLauncher1:", i.UserName)
	if OnLinux() && i.UserName == "root" {
		var rest string
		wd, _ := os.Getwd()
		if zstr.HasPrefix(wd, "/home/", &rest) {
			i.UserName = zstr.HeadUntil(rest, "/")
		}
		// zlog.Info("InstallProgramWithLauncher2:", i.UserName, wd)
	}
	// zlog.Info("InstallProgramWithLauncher2:", i.UserName)
	binDir := i.BinPath(false)
	bin := i.BinPath(true, i.ProductName)
	if copyBinary {
		os.Remove(bin) // delete existing, in case running, and we get busy error
		err := zfile.CopyFile(bin, os.Args[0])
		if zlog.OnError(err, bin, os.Args[0]) {
			return err
		}
	}
	err := os.Chmod(bin, 0700)
	if zlog.OnError(err, bin) {
		return err
	}

	if i.UserName != "" {
		err := zfile.SetOwnerAndMainGroup(bin, i.UserName)
		if zlog.OnError(err, bin) {
			return err
		}
	}
	var launchConfig, launcherPath, serviceTool, sargs string
	if OnDarwin() {
		serviceTool = "launchctl"
		launchConfig = launchAgentPlistStr
		launcherPath = zfile.ExpandTildeInFilepath("~/Library/LaunchAgents/"+i.ID()) + ".plist"
		for _, a := range args {
			sargs += "<string>" + a + "</string>\n"
		}
	} else {
		serviceTool = "systemctl"
		launchConfig = serviceFileStr
		launcherPath = "/lib/systemd/system/" + i.ID() + ".service"
		sargs = strings.Join(args, " ")
	}
	replacer := strings.NewReplacer(
		"{user}", i.UserName,
		"{bin}", bin,
		"{log}", i.VarPath(true, "log.txt"),
		"{id}", i.ID(),
		"{bindir}", binDir,
		"{args}", sargs,
	)
	zlog.Info("WriteServiceFile:", launcherPath)
	launchConfig = replacer.Replace(launchConfig)
	err = zfile.WriteStringToFile(launchConfig, launcherPath)
	zlog.Info("install launcher", launcherPath, zfile.Exists(launcherPath))
	if err != nil {
		return zlog.Error(err, "install launcher", launcherPath, zfile.Exists(launcherPath))
	}
	// if OnLinux() && userName != "root" && userName != "" {
	// 	zfile.SetOwnerAndMainGroup(launcherPath, userName)
	// }
	_, err = RunCommand(serviceTool, 5, "load", launcherPath)
	zlog.OnError(err, launcherPath)
	id := i.ID()
	str, err := RunCommand(serviceTool, 5, "stop", id)
	zlog.OnError(err, str)
	str, err = RunCommand(serviceTool, 5, "start", id)
	zlog.OnError(err, str)
	return nil
}
