package zcommand

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type BrowserType string

const (
	Safari BrowserType = "safari"
	Chrome             = "chrome"
	Edge               = "edge"
)

func GetAppNameOfBrowser(btype BrowserType, fullName bool) string {
	switch btype {
	case Safari:
		return "Safari"
	case Chrome:
		return "Google Chrome"
	case Edge:
		if fullName {
			return "Microsoft Edge Canary"
		}
		return "Microsoft Edge"
	}
	return ""
}

func RunCommand(command string, timeoutSecs float64, args ...string) (string, error) {
	var cmd *exec.Cmd
	// zlog.Info("RunCommand:", command, args)
	var cancel context.CancelFunc
	if timeoutSecs != 0 {
		var ctx context.Context
		// zlog.Info("RunCommand with timeout:", timeoutSecs)
		ctx, cancel = context.WithTimeout(context.Background(), ztime.SecondsDur(timeoutSecs))
		cmd = exec.CommandContext(ctx, command, args...)
	} else {
		cmd = exec.Command(command, args...)
	}
	output, err := cmd.CombinedOutput()
	str := strings.Replace(string(output), "\n", "", -1)
	if err != nil {
		var out string
		for _, a := range args {
			out += "'" + a + "' "
		}
		zlog.Error(err, "Run Command err", "'"+command+"'", out, str)
	}
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
	cmd = exec.Command(path, args...)
	// fmt.Println("RunApp:", path, args)
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
	err = cmd.Start()
	if err != nil {
		err = zlog.Error(err, "run")
		return
	}
	return
}

func RunAppleScript(command string, timeoutSecs float64) (string, error) {
	return RunCommand("osascript", timeoutSecs, "-s", "o", "-e", command)
}

func CloseUrlInBrowser(surl string, btype BrowserType) error {
	command := `
		display alert "hello"
		repeat with w in windows
			repeat with t in tabs of w
				set u to URL of t
				if u is equal to "%s" then 
					close t
					return
				end if
           end repeat
       end repeat
	`
	command = fmt.Sprintf(command, surl)
	_, err := RunAppleScript(command, 10.0)
	return err
}

func CloseAllWIndowsInApp(app string) error {
	command := `tell application "%s"
	activate
	close every window
end tell`
	command = fmt.Sprintf(command, app)
	_, err := RunAppleScript(command, 10.0)
	return err
}

func RunURLInBrowser(surl string, btype BrowserType, args ...string) (*exec.Cmd, error) {
	args = append(args, "--new-window", surl)
	name := GetAppNameOfBrowser(btype, true)
	cmd, _, _, err := RunApp(name, args...)
	if err != nil {
		return nil, zlog.Error(err, "RunURLInBrowser")
	}
	return cmd, err
}

func OpenUrlInBrowser(surl string, btype BrowserType, args ...string) error {
	name := GetAppNameOfBrowser(btype, true)
	args = append([]string{"-F", "-g", "-a", name, surl, "--args", "--new-window"}, args...)
	_, err := RunCommand("open", 0, args...)
	// fmt.Println("OpenUrlInBrowser:", err, args)
	if err != nil {
		return zlog.Error(err, "OpenUrlInBrowser")
	}
	return err
}

func SetMacComment(filepath, comment string) (string, error) {
	format := `tell application "Finder" to set the comment of (the POSIX file "%s" as alias) to "%s"`
	command := fmt.Sprintf(format, filepath, comment)
	str, err := RunAppleScript(command, 5.0)
	return str, err
}

func CaptureScreen(windowID, outputPath string) error {
	_, err := RunCommand("screencapture", 0, "-o", "-x", "-l", windowID, outputPath) // -o is no shadow, -x is no sound, -l is window id
	return err
}

func QuitBrowser(btype BrowserType) (string, error) {
	name := GetAppNameOfBrowser(btype, true)
	str, err := RunCommand("killall", 0, name)
	return str, err
}

func CloseAppWindowWithTitle(app, title string, allExcept bool) error {
	titleName := "title"
	if app == GetAppNameOfBrowser(Safari, true) {
		titleName = "name"
	}
	command :=
		`tell application "%s"
		activate
		close (every window whose %s is %s "%s")
		end tell
`
	not := ""
	if allExcept {
		not = "not"
	}
	command = fmt.Sprintf(command, app, titleName, not, title)
	// fmt.Println("CloseWindowWithTitle:", app, title, "\n", command)
	_, err := RunAppleScript(command, 5.0)
	//	fmt.Println("CloseWindowWithTitle done", err)
	return err
}

func QuitApp(app string) error {
	command :=
		`tell application "%s"
			quit
		end tell
`
	command = fmt.Sprintf(command, app)
	_, err := RunAppleScript(command, 3.0)
	return err
}

func ResizeAppWindowWithTitle(app, title string, size zgeo.Size) error {
	titleName := "title"
	if app == GetAppNameOfBrowser(Safari, true) {
		titleName = "name"
	}
	command :=
		`tell application "%s"
		activate
	    repeat with w in windows
			if %s of w is "%s" then
				set b to bounds of w
				set x to 1st item of b
				set y to 2nd item of b
				log b
				log (title of w)
				log x
				log y
				set bounds of w to {x, y, x + %d, y + %d}
			end if
		end repeat
	end tell
`
	command = fmt.Sprintf(command, app, titleName, title, int(size.W), int(size.H))
	_, err := RunAppleScript(command, 5.0)
	// fmt.Println("ResizeAppWindowWithTitle", command, str, err)
	return err
}

func ResizeBrowserWindowWithTitle(btype BrowserType, title string, rect zgeo.Rect) error {
	titleName := "title"
	if btype == Safari {
		titleName = "name"
	}
	app := GetAppNameOfBrowser(btype, true)
	command :=
		`tell application "%s"
		activate
		set bounds of every window whose %s is "%s" to {%g,%g,%g,%g}
		end tell
`
	command = fmt.Sprintf(command, app, titleName, title, rect.Min().X, rect.Min().Y, rect.Max().X, rect.Max().Y)
	// fmt.Println("CloseWindowWithTitle:", app, title, "\n", command)
	_, err := RunAppleScript(command, 5.0)
	//	fmt.Println("CloseWindowWithTitle done", err)
	return err
}
