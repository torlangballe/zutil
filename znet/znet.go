package znet

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/torlangballe/zutil/zlog"
)

type Organization struct {
	Organization  string
	Country       string
	Province      string
	Locality      string
	StreetAddress string
	PostalCode    string
}

type SSLCertificateInfo struct {
	Organization
	YearsUntilExpiry int
}

var NotFound = errors.New("not found")

func HostAndPortToAddress(host string, port int) string {
	str := host
	if port != 0 {
		str += fmt.Sprint(":", port)
	}
	return str
}

func HostAndPortFromAddress(address string) (string, int) {
	if !strings.ContainsAny(address, ":") {
		return address, 0
	}
	shost, sport, err := net.SplitHostPort(address)
	zlog.OnError(err)
	port, err := strconv.Atoi(sport)
	zlog.OnError(err)
	return shost, port
}

func StripQueryAndFragment(surl string) string {
	u, err := url.Parse(surl)
	if zlog.OnError(err) {
		return surl
	}
	u.RawQuery = ""
	u.RawFragment = ""
	return u.String()
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

func DNSLookupToIP4(domain string) (string, error) {
	ips, err := net.LookupIP(domain)
	if err != nil {
		return "", zlog.NewError(err, domain)
	}
	for _, ip := range ips {
		if ip.To4() != nil {
			return ip.String(), nil
		}
	}
	return "", zlog.Error("DNSLookupToIP4 to ip4 failed", ips, domain)
}
