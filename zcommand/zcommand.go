package zcommand

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/torlangballe/zutil/ustr"

	"github.com/torlangballe/zutil/zlog"

	"github.com/torlangballe/zgo"
)

type BrowserType string

const (
	Safari BrowserType = "safari"
	Chrome             = "chrome"
	Edge               = "edge"
)

func GetAppNameOfBrowser(btype BrowserType) string {
	switch btype {
	case Safari:
		return "Safari"
	case Chrome:
		return "Google Chrome"
	case Edge:
		return "Microsoft Edge Canary"
	}
	return ""
}

func RunCommand(command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)

	output, err := cmd.CombinedOutput()
	str := strings.Replace(string(output), "\n", "", -1)
	if err != nil {
		fmt.Println("Run Command", "'"+command+"'", args, "err:", ustr.EscRed, err, str, ustr.EscNoColor)
	}
	return str, err
}

func RunAppleScript(command string) (string, error) {
	return RunCommand("osascript", "-s", "o", "-e", command)
}

func CloseUrlInBrowser(surl string, btype BrowserType) error {
	fmt.Println()
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
	name := GetAppNameOfBrowser(btype)
	str, err := RunCommand("open", "-F", "-g", "-a", name, surl, "--args", "--new-window")
	if err != nil {
		return str, zlog.Error(err, "OpenUrlInBrowser")
	}
	return str, err
}

func QuitBrowser(btype BrowserType) (string, error) {
	name := GetAppNameOfBrowser(btype)
	str, err := RunCommand("killall", name)
	return str, err
}

func CloseWindowWithTitle(app, title string) error {
	command :=
		`tell application "%s"
		close (every window whose title contains "%s")
		end tell
`
	command = fmt.Sprintf(command, app, title)
	_, err := RunAppleScript(command)
	//	fmt.Println("CloseWindowWithTitle:", app, title, str, err, "\n", command)
	return err
}

func GetWindowId(app, title string) (string, error) {
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

func CropImage(path string, rect zgo.Rect, maxSize zgo.Size) error {
	srect := fmt.Sprintf("%dx%d+%d+%d", int(rect.Size.W), int(rect.Size.H), int(rect.Pos.X), int(rect.Pos.Y))
	args := []string{
		path,
		"-crop",
		srect,
	}
	fmt.Println("CROP:", srect, path)
	if !maxSize.IsNull() {
		ssize := fmt.Sprintf("%dx%d", int(maxSize.W), int(maxSize.H))
		args = append(args, "+repage", "-resize", ssize, "+repage")
	}
	args = append(args, path)
	_, err := RunCommand("convert", args...)
	return err
}

func ProcessImage(inputPath string, commands ...interface{}) error {
	var args []string
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
			r := v.(zgo.Rect)
			srect := fmt.Sprintf("%dx%d+%d+%d", int(r.Size.W), int(r.Size.H), int(r.Pos.X), int(r.Pos.Y))
			args = append(args, "-crop", srect)
		case "resize":
			s := v.(zgo.Size)
			ssize := fmt.Sprintf("%dx%d", int(s.W), int(s.H))
			args = append(args, "-resize", ssize)
		case "comment":
			text := v.(string)
			args = append(args, "-set", "comment", text)
		}
	}
	args = append(args, inputPath)
	_, err := RunCommand("mogrify", args...)
	return err
}
