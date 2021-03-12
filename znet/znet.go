package znet

import (
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zlog"
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
