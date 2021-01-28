package zhost

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
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

// func GetCurrentLocalIPAddress2() (ip16, ip4 string, err error) {
// 	addrs, err := net.InterfaceAddrs()
// 	// zlog.Info("CurrentLocalIP Stuff:", addrs, err)
// 	if err != nil {
// 		return
// 	}

// 	for _, a := range addrs {
// 		ipnet, ok := a.(*net.IPNet)
// 		if ok {
// 			if ipnet.IP.IsLoopback() {
// 				continue
// 			}
// 			i16 := ipnet.IP.To16()
// 			if i16 != nil {
// 				ip16 = i16.String()
// 			}
// 			i4 := ipnet.IP.To4()
// 			if i4 != nil {
// 				ip4 = i4.String()
// 				zlog.Info("IP:", a.String(), ip4)
// 				break
// 			}
// 		}
// 	}
// 	return
// }

func GetCurrentLocalIPAddress() (ip16, ip4 string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	var oldName string
	var oldNum int = -1
	for _, iface := range ifaces {
		// zlog.Info("CurrentLocalIP Stuff:", iface)
		addresses, e := iface.Addrs()
		if e != nil {
			err = e
			return
		}
		for _, a := range addresses {
			ipnet, ok := a.(*net.IPNet)
			if ok {
				if ipnet.IP.IsLoopback() {
					continue
				}
				get := false
				name := iface.Name
				// zlog.Info("CurrentLocalIP device:", name)
				var snum string
				win := (runtime.GOOS == "windows")

				if oldName == "" || (!win && zstr.HasPrefix(name, "en", &snum) || zstr.HasPrefix(name, "eth", &snum)) ||
					win && name == "Ethernet" {
					if oldName == "" || (!strings.HasPrefix(oldName, "en") && !strings.HasPrefix(oldName, "eth")) {
						oldName = name
						get = true
					} else {
						num, _ := strconv.Atoi(snum)
						if num >= oldNum {
							get = true
						}
						oldNum = num
					}
				}
				if get {
					i16 := ipnet.IP.To16()
					if i16 != nil {
						ip16 = i16.String()
					}
					i4 := ipnet.IP.To4()
					if i4 != nil {
						str := i4.String()
						// zlog.Info("IP:", a.String(), ip4, iface.Name, str)
						if strings.HasPrefix(str, "169.") && ip4 != "" {
							continue
						}
						ip4 = str
					}
				}
			}
		}
	}
	return
}

func GetOutboundIP() (ip net.IP, err error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip = localAddr.IP
	return
}

func GetCurrentIPAddress() (address string, err error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				return v.String(), nil
			case *net.IPAddr:
				return v.String(), nil
			}
			// process IP address
		}
	}
	return "", nil
}

func ForwardPortToRemote(port int, remoteAddress string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return zlog.Error(err, "listen")
	}
	zlog.Info("forwarder running on", port, "to", remoteAddress)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return zlog.Error(err, "accept")
		}
		forwardPortHandleRequest(conn, remoteAddress)
	}
}

func forwardPortHandleRequest(conn net.Conn, remoteAddress string) {
	proxy, err := net.Dial("tcp", remoteAddress)
	if err != nil {
		zlog.Error(err, "dial target")
		return
	}
	go copyIO(conn, proxy)
	go copyIO(proxy, conn)
}

func copyIO(src, dest net.Conn) {
	defer src.Close()
	defer dest.Close()
	io.Copy(src, dest)
}

func GetHostAndPort(u *url.URL) (host string, port int) {
	var err error
	var sport string
	if !strings.Contains(u.Host, ":") {
		return u.Host, 0
	}
	host, sport, err = net.SplitHostPort(u.Host)
	if err != nil {
		zlog.Error(err)
		return
	}
	port, err = strconv.Atoi(sport)
	if err != nil {
		zlog.Error(err)
		return
	}
	return
}
