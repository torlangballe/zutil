//go:build !js

package zprocess

import (
	"os"
	"strings"

	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
)

type Installer struct {
	Company     string
	ProductName string
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
        <string>{var}/log.txt</string>
        <key>StandardErrorPath</key>
        <string>{var}/log.txt</string>
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

func (i *Installer) OptPath(add string) string {
	return zfile.JoinPathParts("/opt", i.Company, i.ProductName, add)
}

func (i *Installer) BinPath(add string) string {
	return zfile.JoinPathParts(i.OptPath("bin"), add)
}

func (i *Installer) VarPath(add string) string {
	return zfile.JoinPathParts(i.OptPath("etc/var"), add)
}

func (i *Installer) ID() string {
	return i.Company + "." + i.ProductName
}

func (i *Installer) InstallProgramWithLauncher(args []string, copyBinary bool) error {
	binDir := i.BinPath("")
	zfile.MakeDirAllIfNotExists(binDir)
	bin := binDir + "/" + i.ProductName
	if copyBinary {
		err := zfile.CopyFile(bin, os.Args[0])
		if err != nil {
			return err
		}
	}
	var launchConfig, launcherPath, sargs string
	if OnDarwin() {
		launchConfig = launchAgentPlistStr
		launcherPath = zfile.ExpandTildeInFilepath("~/Library/LaunchAgents/"+i.ID()) + ".plist"
	}
	for _, a := range args {
		sargs += "<string>" + a + "</string>\n"
	}

	replacer := strings.NewReplacer(
		"{bin}", bin,
		"{var}", i.VarPath(""),
		"{id}", i.ID(),
		"{bindir}", binDir,
		"{args}", sargs,
	)
	launchConfig = replacer.Replace(launchConfig)
	err := zfile.WriteStringToFile(launchConfig, launcherPath)
	if err != nil {
		return zlog.Error(err, "install launcher", launcherPath)
	}
	const launch = "launchctl"
	_, err = RunCommand(launch, 5, "load", launcherPath)
	if err != nil {
		RunCommand(launch, 5, "stop")
		return nil
	} else {
		RunCommand(launch, 5, "start")
	}
	return nil
}
