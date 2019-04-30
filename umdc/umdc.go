package umdc

import (
	"btech/mdc/packet"
	"btech/mdc/utime"
	"fmt"
	"net"
	"time"
	"zutil/unet"

	"github.com/pkg/errors"
)

type portRangeType struct {
	start net.IPAddr
	end   net.IPAddr
	port  int
}

type Probe struct {
	mdcUdpAddress *net.UDPAddr
	mdcClient     *packet.Client
	conn          *net.UDPConn
	stopped       bool
	ipRange       []portRangeType
	newIpRange    []portRangeType
	ipIndex       int
	ipNext        net.IPAddr
	ipWrapped     bool
	CallbackInfo1 interface{}
	CallbackInfo2 interface{}
	CallbackInfo3 interface{}

	downloadView *packet.View
	cpuView      *packet.View
	trafficView  *packet.View

	HandleGotNextMulticast func(p *Probe, ip net.IPAddr, port int)
}

func (p *Probe) StartNextTest() {
	if len(p.ipRange) > 0 || len(p.newIpRange) > 0 {
		p.nextInRange()
	} else {
		fmt.Println("probe.StartNextTest: no range!!!")
	}
}

func (p *Probe) reader(conn *net.UDPConn) {
	var buf [2048]byte
	for {
		n, err := conn.Read(buf[:])
		if err != nil {
			if !p.stopped {
				fmt.Println("reader con.Read err:", err)
			}
			return
		}
		pack, err := packet.Open(buf[:n])
		if err != nil {
			fmt.Println("reader packet.Open err:", err)
			return
		}
		fmt.Printf("packet received: %v\n", p)
		//		fmt.Printf("parts received: %v\n", p.GetData("CWE"))
		wrange := pack.GetData("CW")
		if len(p.ipRange) == 0 {
			p.handleIPRange(wrange)
		}
	}
}

func (p *Probe) nextInRange() {
	p.increaseIpRange()
	port := p.ipRange[p.ipIndex].port
	p.HandleGotNextMulticast(p, p.ipNext, port)
}

func (p *Probe) increaseIpRange() {
	if p.ipIndex >= len(p.ipRange) || unet.IpToInt32(p.ipNext.IP) >= unet.IpToInt32(p.ipRange[p.ipIndex].end.IP) {
		p.ipIndex++
		if p.ipIndex >= len(p.ipRange) {
			if len(p.newIpRange) > 0 {
				p.ipRange = p.newIpRange
				p.newIpRange = p.newIpRange[:0]
				//				fmt.Println("setnewrange:", len(p.ipRange))
			}
			p.ipIndex = 0
		}
		//		fmt.Println("incrange:", p.ipIndex, "in", len(p.ipRange))
		p.ipNext = p.ipRange[p.ipIndex].start
		return
	}
	c := unet.IpToInt32(p.ipNext.IP)
	p.ipNext.IP = unet.Int32ToIp(c + 1)
	//	fmt.Println("increaseIpRange to:", mms.ipNext.IP)
}

func (p *Probe) handleIPRange(wrange packet.Reader) {
	var ipRange []portRangeType
	for {
		tag, data := wrange.Read()
		if tag == 0 {
			break
		}
		if tag != 'E' || len(data) != 10 {
			fmt.Println("reader: not good length E tag in CW", tag, len(data))
			continue
		}
		ipstart := net.IPv4(data[0], data[1], data[2], data[3])
		ipend := net.IPv4(data[4], data[5], data[6], data[7])
		port := data[8:10].I16()
		var ip portRangeType
		ip.start = net.IPAddr{IP: ipstart}
		ip.end = net.IPAddr{IP: ipend}
		ip.port = port
		ipRange = append(ipRange, ip)
		p.newIpRange = ipRange
	}
	p.nextInRange()
}

func NewProbe(uniqueId uint64, mdcAddress string, mdcPort int) (p *Probe, err error) {
	p = &Probe{}
	ip, err := net.ResolveIPAddr("ip4", mdcAddress)
	if err != nil {
		err = errors.Wrap(err, "multicast.NewProbe Resolve")
		return
	}
	p.mdcUdpAddress = &net.UDPAddr{IP: ip.IP, Port: mdcPort}
	p.conn, err = net.DialUDP("udp", nil, p.mdcUdpAddress)

	if err != nil {
		err = errors.Wrap(err, ".NewProbe net.DialUDP1")
		return
	}
	p.mdcClient = packet.NewClient(uniqueId, p.conn)
	go p.reader(p.conn)

	return
}

func (p *Probe) Stop() {
	p.stopped = true
	if p.mdcClient != nil {
		p.mdcClient.Close()
	}
	if p.conn != nil {
		p.conn.Close()
		p.conn = nil
	}
}

func getUdpIp(address string, port int) (ip *net.UDPAddr, err error) {
	ip4, err := net.ResolveIPAddr("ip4", address)
	if err != nil {
		err = errors.Wrap(err, "getUdpIp Resolve")
		return
	}
	ip = &net.UDPAddr{IP: ip4.IP, Port: port}
	return
}

func (p *Probe) makeMulticastView(start time.Time, dest, source string, joinlat float64) *packet.View {
	ttl := 5
	tos := 111

	d, _ := net.ResolveUDPAddr("udp4", dest)
	s, _ := net.ResolveUDPAddr("udp4", source)

	fmt.Println("mdc makemultiview resolve ip:", source, s, dest, d, p)
	j := utime.Duration(joinlat) * utime.Millisecond
	return p.mdcClient.NewMulticastView(utime.Now(), *d, s.IP, j, ttl, tos)
}

func sendStream(view *packet.View, bwSamples []float64, mlrSamples []float64) {
	bwInt := make([]int, len(bwSamples))
	for i, s := range bwSamples {
		bwInt[i] = int(s)
	}

	stream := packet.Stream{
		BW: bwInt,
	}
	if len(mlrSamples) > 0 {
		mlrInt := make([]int, len(mlrSamples))
		for i, s := range mlrSamples {
			mlrInt[i] = int(s)
		}
		stream.MLR = mlrInt
	}
	view.WriteStream(&stream)
	return
}

func (p *Probe) SendDownloadStream(start time.Time, dest, source string, joinlat float64, bwSamples, mlrSamples []float64) (err error) {
	fmt.Println("SendDownloadStream:", source, dest)
	if p.downloadView == nil {
		p.downloadView = p.makeMulticastView(start, dest, source, joinlat)
	}
	sendStream(p.downloadView, bwSamples, mlrSamples)
	return
}

func (p *Probe) SendTrafficLanStream(start time.Time, source string, samples []float64) (err error) {
	fmt.Println("SendTrafficLanStream")
	dest := "0.0.0.1:1"

	if p.trafficView == nil {
		p.trafficView = p.makeMulticastView(start, dest, source, 0.0)
	}
	sendStream(p.trafficView, samples, nil)
	return
}

func (p *Probe) SendCpuStream(start time.Time, source string, samples []float64) (err error) {
	fmt.Println("SendCpuStream")
	dest := "0.0.0.2:1"
	if p.cpuView == nil {
		p.cpuView = p.makeMulticastView(start, dest, source, 0.0)
	}
	sendStream(p.cpuView, nil, samples)
	return
}
