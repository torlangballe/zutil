package uhost

import (
	"io/ioutil"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
)

const osxCmd = "/System/Library/PrivateFrameworks/Apple80211.framework/Versions/Current/Resources/airport"
const osxArgs = "-I"
const linuxCmd = "iwgetid"
const linuxArgs = "--raw"

func WifiName() (name string, err error) {
	platform := runtime.GOOS
	if platform == "darwin" {
		return forOSX()
	} else if platform == "win32" {
		// TODO for Windows
		return
	} else {
		// TODO for Linux
		return forLinux()
	}
}

func forLinux() (name string, err error) {
	cmd := exec.Command(linuxCmd, linuxArgs)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	// start the command after having set up the pipe
	err = cmd.Start()
	if err != nil {
		return
	}

	var str string

	if b, err := ioutil.ReadAll(stdout); err == nil {
		str += (string(b) + "\n")
	}

	name = strings.Replace(str, "\n", "", -1)
	return
}

func forOSX() (name string, err error) {

	cmd := exec.Command(osxCmd, osxArgs)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}

	// start the command after having set up the pipe
	err = cmd.Start()
	if err != nil {
		return
	}

	var str string

	if b, err := ioutil.ReadAll(stdout); err == nil {
		str += (string(b) + "\n")
	}

	r := regexp.MustCompile(`s*SSID: (.+)s*`)

	names := r.FindAllStringSubmatch(str, -1)

	if len(names) <= 1 {
		name = "Could not get SSID"
	} else {
		name = names[1][1]
	}
	return
}
