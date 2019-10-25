package unet

import (
	"encoding/binary"
	"net"
	//	"github.com/tatsushid/go-fastping"
)

func IpToInt32(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func Int32ToIp(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

func ParseMACToBigEndian(mac string) (uint64, error) {
	b, err := net.ParseMAC(mac)
	if err != nil {
		return 0, err
	}
	b = append(make([]byte, 8-len(b)), b...)
	return binary.BigEndian.Uint64(b), nil
}

// // Provide either a net.IPAddr or saddress domain name/ip
// func Ping(address *net.IPAddr, saddress string) (latency time.Duration, err error) {
// 	p := NewPinger()
// 	if saddress != "" {
// 		address, err = net.ResolveIPAddr("ip4:icmp", saddress)
// 		if err != nil {
// 			fmt.Println(err)
// 			return
// 		}
// 	}
// 	p.AddIPAddr(address)
// 	p.Network("udp")
// 	// p.OnRecv = func(addr *net.IPAddr, rtt time.Duration) {
// 	// 	latency = rtt
// 	// 	fmt.Printf("IP Addr: %s receive, RTT: %v\n", addr.String(), rtt)
// 	// 	p.Done()
// 	// }
// 	// p.OnIdle = func() {
// 	// 	fmt.Println("OnIdle")
// 	// }
// 	err = p.Run(5)
// 	return
// }

func GetMACAddresses() (map[string]string, error) {
	ifas, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	as := map[string]string{}
	for _, ifa := range ifas {
		a := ifa.HardwareAddr.String()
		if a == "" {
			continue
		}
		if ifa.Flags&net.FlagLoopback != 0 {
			continue
		}
		as[ifa.Name] = a
	}
	return as, nil
}
