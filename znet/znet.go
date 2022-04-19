package znet

import (
	"net"
	"net/url"
	"runtime"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
)

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
