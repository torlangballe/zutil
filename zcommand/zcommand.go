package zcommand

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
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

func RunCommand(command string, args ...string) (string, error) {
	//	zlog.Info("RunCommand:", command, args)
	cmd := exec.Command(command, args...)
	output, err := cmd.CombinedOutput()
	str := strings.Replace(string(output), "\n", "", -1)
	if err != nil {
		var out string
		for _, a := range args {
			out += "'" + a + "' "
		}
		zlog.Error(err, "Run Command err", "'"+command+"'", out, str)
	}
	return str, err
}

func RunAppleScript(command string) (string, error) {
	return RunCommand("osascript", "-s", "o", "-e", command)
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
	_, err := RunAppleScript(command)
	return err
}

func OpenUrlInBrowser(surl string, btype BrowserType) (string, error) {
	if btype == Chrome {
		path := "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
		str, err := RunCommand(path, "--new-window", surl)
		return str, err
	}
	name := GetAppNameOfBrowser(btype, true)
	str, err := RunCommand("open", "-F", "-g", "-a", name, surl, "--args", "--new-window")
	if err != nil {
		return str, zlog.Error(err, "OpenUrlInBrowser")
	}
	return str, err
}

func QuitBrowser(btype BrowserType) (string, error) {
	name := GetAppNameOfBrowser(btype, true)
	str, err := RunCommand("killall", name)
	return str, err
}

func CloseBrowserWindowWithTitle(btype BrowserType, title string) error {
	titleName := "title"
	if btype == Safari {
		titleName = "name"
	}
	app := GetAppNameOfBrowser(btype, true)
	command :=
		`tell application "%s"
		close (every window whose %s contains "%s")
		end tell
`
	command = fmt.Sprintf(command, app, titleName, title)
	_, err := RunAppleScript(command)
	//	fmt.Println("CloseWindowWithTitle:", app, title, str, err, "\n", command)
	return err
}

func GetWindowID(app, title string) (string, error) {
	str, err := RunCommand("getwindowid", app, title)
	if err != nil {
		str = strings.TrimSpace(str)
	}
	return str, err
}

func SetMacComment(filepath, comment string) (string, error) {
	format := `tell application "Finder" to set the comment of (the POSIX file "%s" as alias) to "%s"`
	command := fmt.Sprintf(format, filepath, comment)
	str, err := RunAppleScript(command)
	return str, err
}

func CaptureScreen(windowID, outputPath string) error {
	_, err := RunCommand("screencapture", "-o", "-x", "-l", windowID, outputPath) // -o is no shadow, -x is no sound, -l is window id
	return err
}

func CropImage(path string, rect zgeo.Rect, maxSize zgeo.Size) error {
	srect := fmt.Sprintf("%dx%d+%d+%d", int(rect.Size.W), int(rect.Size.H), int(rect.Min().X), int(rect.Min().Y))
	args := []string{
		"mogrify",
		path,
		"-crop",
		srect,
	}
	if !maxSize.IsNull() {
		ssize := fmt.Sprintf("%dx%d", int(maxSize.W), int(maxSize.H))
		args = append(args, "+repage", "-resize", ssize, "+repage")
	}
	args = append(args, path)
	_, err := RunCommand("magick", args...)
	return err
}

func ProcessImage(inputPath string, commands ...interface{}) error {
	var args = []string{"mogrify"}
	for i := 0; i < len(commands); i++ {
		v := commands[i]
		c, got := v.(string)
		if !got {
			return zlog.Error(nil, "didn't get image command", i, v)
		}
		i++
		v = commands[i]
		switch c {
		case "crop":
			r := v.(zgeo.Rect)
			srect := fmt.Sprintf("%dx%d+%d+%d", int(r.Size.W), int(r.Size.H), int(r.Min().X), int(r.Min().Y))
			args = append(args, "-crop", srect)
		case "resize":
			s := v.(zgeo.Size)
			ssize := fmt.Sprintf("%dx%d", int(s.W), int(s.H))
			args = append(args, "-resize", ssize)
		case "comment":
			text := v.(string)
			args = append(args, "-set", "comment", text)
		}
	}
	args = append(args, inputPath)
	_, err := RunCommand("magick", args...)
	return err
}
