package znet

import (
	"fmt"
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

func GetIP4AddressAsParts(address string) (parts []int, err error) {
	sparts := strings.Split(address, ".")
	if len(sparts) != 4 {
		return nil, zlog.NewError("wrong number of parts:", len(sparts), address)
	}
	for _, p := range sparts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		if n < 0 || n > 255 {
			return nil, zlog.NewError("number out of range:", n)
		}
		parts = append(parts, n)
	}
	return
}

func GetIP4PartsAsAddress(parts []int) (address string, err error) {
	if len(parts) != 4 {
		return "", zlog.NewError("wrong number of parts:", len(parts))
	}
	for _, n := range parts {
		if n < 0 || n > 255 {
			return "", zlog.NewError("number out of range:", n)
		}
	}
	address = fmt.Sprintf("%d.%d.%d.%d", parts[0], parts[1], parts[2], parts[3])
	return
}
