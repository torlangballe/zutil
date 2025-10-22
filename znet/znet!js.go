//go:build !js

package znet

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/torlangballe/zutil/zdebug"
	"github.com/torlangballe/zutil/zdict"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zprocess"
	"github.com/torlangballe/zutil/zstr"
)

type ZNetCalls struct{}

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

// TODO: Consolidate with with znet.go variants
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

func ForwardPortToRemote(port int, remoteAddress string) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return zlog.Error("listen", err)
	}
	zlog.Info("forwarder running on", port, "to", remoteAddress)
	for {
		conn, err := listener.Accept()
		if err != nil {
			return zlog.Error("accept", err)
		}
		proxy, err := net.Dial("tcp", remoteAddress)
		if err != nil {
			zlog.Error("dial target", err)
			continue
		}
		go copyIO(conn, proxy)
		go copyIO(proxy, conn)
	}
}

func copyIO(src, dest net.Conn) {
	defer src.Close()
	defer dest.Close()
	io.Copy(src, dest)
}

// var certManager = autocert.Manager{
// 	Prompt: autocert.AcceptTOS,
// 	Cache:  autocert.DirCache("certs"),
// }

func ShowGenerateTLSCertificatesCommands(certName string) {
	os.Mkdir("certs", 0775|os.ModeDir)
	// Key considerations for algorithm "RSA" ≥ 2048-bit
	zlog.Info("🟨openssl genrsa -out certs/" + certName + ".key 2048")
	// Key considerations for algorithm "ECDSA" ≥ secp384r1
	// List ECDSA the supported curves (openssl ecparam -list_curves)
	zlog.Info("🟨openssl ecparam -genkey -name secp384r1 -out certs/" + certName + ".key")
	//	Generation of self-signed(x509) public key (PEM-encodings .pem|.crt) based on the private (.key)
	zlog.Info("🟨openssl req -new -x509 -sha256 -key certs/" + certName + ".key -out certs/" + certName + ".crt -days 3650")
}

type HTTPServer struct {
	Server      *http.Server
	doneChannel chan bool
}

func ServeHTTPInBackground(address string, certificatesStubPath string, handler http.Handler) (server *HTTPServer, certificateExpires time.Time) {
	// https://ap.www.namecheap.com/Domains/DomainControlPanel/etheros.online/advancedns
	// https://github.com/denji/golang-tls
	//
	str := "Serve HTTP"
	if certificatesStubPath != "" {
		str += "S"
	}
	host, sport, err := net.SplitHostPort(address)
	if err == nil {
		if host == "127.0.0.1" {
			address = ":" + sport
		}
		if sport == "" {
			if certificatesStubPath != "" {
				sport = "443"
			} else {
				sport = "80"
			}
			address = host + ":" + sport
		}
	}
	stack := zdebug.CallingStackString()
	s := &HTTPServer{}
	s.Server = &http.Server{Addr: address}
	s.Server.Handler = handler
	s.Server.ReadTimeout = 60 * time.Second
	s.Server.WriteTimeout = 60 * time.Second
	s.Server.TLSConfig = &tls.Config{
		InsecureSkipVerify: true,
		// ClientAuth: tls.RequireAndVerifyClientCert, -- this was to test if clients need certificates
	}
	s.doneChannel = make(chan bool, 100)
	var fCRT, fKey string
	if certificatesStubPath != "" {
		fCRT = certificatesStubPath + ".crt"
		fKey = certificatesStubPath + ".key"
		if zfile.NotExists(fCRT) || zfile.NotExists(fKey) {
			zlog.Error("missing certificate files:", fCRT, fKey)
			return
		}
		cer, err := tls.LoadX509KeyPair(fCRT, fKey)
		if !zlog.OnError(err, "LoadX509KeyPair") {
			x509Cert, err := x509.ParseCertificate(cer.Certificate[0])
			if !zlog.OnError(err, "ParseCertificate") {
				certificateExpires = x509Cert.NotAfter
			}
		}
	}
	go func() {
		if certificatesStubPath != "" {
			err = s.Server.ListenAndServeTLS(fCRT, fKey)
		} else {
			err = s.Server.ListenAndServe()
		}
		if err != http.ErrServerClosed {
			if err != nil {
				zlog.Error("serve http listen err:", address, certificatesStubPath, stack, err)
				os.Exit(-1)
			}
		}
		s.doneChannel <- true
	}()
	return s, certificateExpires
}

func (s *HTTPServer) Shutdown(wait bool) error {
	err := s.Server.Shutdown(context.TODO())
	if err != nil {
		return err
	}
	if wait {
		<-s.doneChannel
	}
	return nil
}

// SetEtcHostsEntry adds a 1.2.3.4 example.com #comment line to /etc/hosts
// Any line with the same comment is removed first.
// If ip or domain are empty, no line is added.
// It requires the running user to be able to sudo, and run a shell
func SetEtcHostsEntries(forwards map[string]string, comment, sudoPassword string) error {
	hpath := "/etc/hosts"
	sed := fmt.Sprintf(`sed -i .old '/%s/d' %s`, comment, hpath)
	var echos []string
	for domain, ip := range forwards {
		line := fmt.Sprintf(`echo '%s %s %s' >> %s`, ip, domain, comment, hpath)
		echos = append(echos, line)
	}
	sechos := strings.Join(echos, " ; ")
	commands := sed + " ; " + sechos
	zlog.Info("SetEtcHostsEntries:", commands)
	str, err := zprocess.RunCommandWithSudo("sh", sudoPassword, "-c", commands)
	if err != nil {
		return zlog.NewError(str, err)
	}
	return err
}

func BindPort() error {
	str, err := zprocess.RunCommand("/usr/sbin/setcap", 5, "CAP_NET_BIND_SERVICE=+eip", "/opt/btech/qtt/bin/manager")
	if err != nil {
		return zlog.Error(str, err)
	}
	return nil
}

const (
	SNMPSysName     = ".1.3.6.1.2.1.1.5.0"
	SNMPName        = ".1.3.6.1.2.1.1.1.0"
	SNMPSysObjectID = ".1.3.6.1.2.1.1.2.0"
	// https://www.10-strike.com/network-monitor/pro/useful-snmp-oids.shtml
)

func GetSNMPVariables(host string, oids ...string) (zdict.Dict, error) {
	client := *gosnmp.Default
	client.Target = host
	client.Timeout = time.Second
	client.Retries = 1
	err := client.Connect()
	if err != nil {
		return nil, err
	}
	defer client.Conn.Close()

	result, err := client.Get(oids) // Get() accepts up to g.MAX_OIDS
	if err != nil {
		return nil, err
	}

	d := zdict.Dict{}
	for _, variable := range result.Variables {
		switch variable.Type {
		case gosnmp.OctetString:
			str := variable.Value.(string)
			d[variable.Name] = str
		default:
			n := gosnmp.ToBigInt(variable.Value)
			d[variable.Name] = n
		}
	}
	return d, nil
}

func GetSNMPVariableStr(host string, oid string) (string, error) {
	vals, err := GetSNMPVariables(host, oid)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(vals[oid]), nil
}

func GetCurrentLocalIP4Address(skipLocal bool, netInterface string) (ip4 string, err error) {
	all, err := GetCurrentLocalIP4Addresses(skipLocal)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", errors.New("no ip4 address")
	}
	if netInterface != "" {
		ip4 = all[netInterface]
		if ip4 != "" {
			return
		}
	}
	err = zmap.GetAnyValue(&ip4, all)
	return ip4, err
}

// GetCurrentLocalIP4Addresses returns a map of interface:ip4-address
func GetCurrentLocalIP4Addresses(skipLocal bool) (map[string]string, error) {
	m := map[string]string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var oldName string
	var oldNum int = -1
	for i, iface := range ifaces {
		addresses, e := iface.Addrs()
		if e != nil {
			return m, e
		}
		for _, a := range addresses {
			ipnet, ok := a.(*net.IPNet)
			if ok {
				// zlog.Info("IP:", a.String(), iface.Name, ipnet.IP.IsLoopback())
				if ipnet.IP.IsLoopback() {
					continue
				}
				get := false
				name := iface.Name
				// zlog.Info("CurrentLocalIP device:", name)
				var snum string
				win := (runtime.GOOS == "windows")

				// code to prefer en/eth interfaces with highest number
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
				if get || i == len(ifaces)-1 {
					// i16 := ipnet.IP.To16()
					// if i16 != nil {
					// 	ip16 = i16.String()
					// }
					i4 := ipnet.IP.To4()
					if i4 != nil {
						str := i4.String()
						// zlog.Info("IP:", a.String(), iface.Name, str)
						if skipLocal && str == "127.0.0.1" { // && (strings.HasPrefix(str, "192.168.")
							continue
						}
						m[iface.Name] = str
					}
				}
			}
		}
	}
	return m, nil
}

func GetInterfaces(ip4, ip6 bool, mask, local bool) (map[string]string, error) {
	m := map[string]string{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				if !local && v.IP.IsLoopback() {
					continue
				}
				if ip6 {
					i16 := v.IP.To16()
					if i16 != nil {
						m[iface.Name] = i16.String()
					}
				}
				if ip4 {
					i4 := v.IP.To4()
					if i4 != nil {
						m[iface.Name] = i4.String()
						// zlog.Info("IP:", a.String(), ip4, iface.Name, str)
					}
				}
			case *net.IPAddr:
				m[iface.Name] = v.String()
			}
		}
	}
	if !mask {
		for k, v := range m {
			k = zstr.HeadUntil(k, "/")
			m[k] = v
		}
	}
	return m, nil
}

func InterfaceOfIP4Address(address string) (string, error) {
	host, _ := HostAndPortFromAddress(address)
	if host == "" {
		return "", nil
	}
	ifaces, err := GetInterfaces(true, false, false, true)
	if err != nil {
		return "", err
	}
	for iface, addr := range ifaces {
		if addr == host {
			return iface, nil
		}
	}
	return "", NotFound
}
