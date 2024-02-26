//go:build !js

package znetstats

import (
	"context"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	manuf "github.com/timest/gomanuf"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztime"
)

type NetStats struct {
	ipNet      *net.IPNet
	localHaddr net.HardwareAddr
	iface      string
	data       map[string]Info
	t          *time.Ticker
	do         chan string

	LookupManufacturer func(mac string) string
}

const (
	START = "start"
	END   = "end"
)

type Info struct {
	Mac      net.HardwareAddr
	Hostname string
	Manuf    string
}

func New() *NetStats {
	n := &NetStats{}
	n.LookupManufacturer = func(mac string) string {
		m := manuf.Search(mac)
		return m
	}
	return n
}

func (n *NetStats) lookupManuf(mac string) string {
	m := manuf.Search(mac)
	if m == "" {
		if n.LookupManufacturer != nil {
			return n.LookupManufacturer(mac)
		}
	}
	return m
}

func (n *NetStats) pushData(ip string, mac net.HardwareAddr, hostname string, manuf string) {
	n.do <- START
	var mu sync.RWMutex
	mu.RLock()
	defer func() {
		n.do <- END
		mu.RUnlock()
	}()
	if _, ok := n.data[ip]; !ok {
		n.data[ip] = Info{Mac: mac, Hostname: hostname, Manuf: manuf}
		return
	}
	info := n.data[ip]
	if len(hostname) > 0 && len(info.Hostname) == 0 {
		info.Hostname = hostname
	}
	if mac != nil {
		info.Mac = mac
	}
	if info.Manuf == "" {
		info.Manuf = manuf
	}
	n.data[ip] = info
}

func (n *NetStats) setupNetInfo(f string) error {
	var ifs []net.Interface
	var err error
	if f == "" {
		ifs, err = net.Interfaces()
	} else {
		var it *net.Interface
		it, err = net.InterfaceByName(f)
		if err == nil {
			ifs = append(ifs, *it)
		}
	}
	if err != nil {
		return zlog.Error(err)
	}
	for _, it := range ifs {
		addr, _ := it.Addrs()
		for _, a := range addr {
			if ip, ok := a.(*net.IPNet); ok && !ip.IP.IsLoopback() {
				if ip.IP.To4() != nil {
					n.ipNet = ip
					n.localHaddr = it.HardwareAddr
					n.iface = it.Name
					goto END
				}
			}
		}
	}
END:
	if n.ipNet == nil || len(n.localHaddr) == 0 {
		return zlog.Error("end")
	}
	return nil
}

func (n *NetStats) localHost() {
	host, _ := os.Hostname()
	n.data[n.ipNet.IP.String()] = Info{Mac: n.localHaddr, Hostname: strings.TrimSuffix(host, ".local"), Manuf: n.LookupManufacturer(n.localHaddr.String())}
}

func (n *NetStats) sendARP() {
	ips := Table(n.ipNet)
	for _, ip := range ips {
		go n.sendArpPackage(ip)
	}
}

func (n *NetStats) CollectStats(iface string, timeoutSecs float64, got func(ip, mac, host, manuf string)) {
	// allow non root user to execute by compare with euid
	if os.Geteuid() != 0 {
		zlog.Fatal("goscan must run as root.")
	}
	n.data = make(map[string]Info)
	n.do = make(chan string)
	n.setupNetInfo(iface)

	ctx, cancel := context.WithCancel(context.Background())
	go n.listenARP(ctx)
	go n.listenMDNS(ctx)
	go n.listenNBNS(ctx)
	go n.sendARP()
	go n.localHost()

	n.t = time.NewTicker(ztime.SecondsDur(timeoutSecs))
	for {
		select {
		case <-n.t.C:
			n.forwardData(got)
			cancel()
			goto END
		case d := <-n.do:
			switch d {
			case START:
				n.t.Stop()
			case END:
				n.t = time.NewTicker(2 * time.Second)
			}
		}
	}
END:
}

func (n *NetStats) forwardData(got func(ip, mac, host, manuf string)) {
	var keys IPSlice
	for k := range n.data {
		keys = append(keys, ParseIPString(k))
	}
	sort.Sort(keys)
	for _, k := range keys {
		d := n.data[k.String()]
		mac := ""
		if d.Mac != nil {
			mac = d.Mac.String()
		}
		got(k.String(), mac, d.Hostname, d.Manuf)
	}
}
