package unet

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	TimeSliceLength  = 8
	ProtocolICMP     = 1
	ProtocolIPv6ICMP = 58
)

var (
	ipv4Proto = map[string]string{"ip": "ip4:icmp", "udp": "udp4"}
	ipv6Proto = map[string]string{"ip": "ip6:ipv6-icmp", "udp": "udp6"}
)

// Pinger represents ICMP packet sender/receiver
type Pinger struct {
	id  int
	seq int
	// key string is IPAddr.String()
	//addrs   map[string]*net.IPAddr
	address *net.IPAddr
	network string
	source  string
	source6 string
	hasIPv4 bool
	hasIPv6 bool
	//	ctx     *context
	//	mu      sync.Mutex

	// Size in bytes of the payload to send
	Size int
	// Number of (nano,milli)seconds of an idle timeout. Once it passed,
	// the library calls an idle callback function. It is also used for an
	// interval time of RunLoop() method
	MaxRTT time.Duration
	// OnRecv is called with a response packet's source address and its
	// elapsed time when Pinger receives a response packet.
	OnRecv func(*net.IPAddr, time.Duration)
	// OnIdle is called when MaxRTT time passed
	OnIdle func()
	// If Debug is true, it prints debug messages to stdout.
	Debug bool

	Quit bool
}

func byteSliceOfSize(n int) []byte {
	b := make([]byte, n)
	for i := 0; i < len(b); i++ {
		b[i] = 1
	}

	return b
}

func timeToBytes(t time.Time) []byte {
	nsec := t.UnixNano()
	b := make([]byte, 8)
	for i := uint8(0); i < 8; i++ {
		b[i] = byte((nsec >> ((7 - i) * 8)) & 0xff)
	}
	return b
}

func bytesToTime(b []byte) time.Time {
	var nsec int64
	for i := uint8(0); i < 8; i++ {
		nsec += int64(b[i]) << ((7 - i) * 8)
	}
	return time.Unix(nsec/1000000000, nsec%1000000000)
}

func isIPv4(ip net.IP) bool {
	return len(ip.To4()) == net.IPv4len
}

func isIPv6(ip net.IP) bool {
	return len(ip) == net.IPv6len
}

func ipv4Payload(b []byte) []byte {
	if len(b) < ipv4.HeaderLen {
		return b
	}
	hdrlen := int(b[0]&0x0f) << 2
	return b[hdrlen:]
}

type packet struct {
	bytes []byte
	addr  net.Addr
}

// NewPinger returns a new Pinger struct pointer
func NewPinger() *Pinger {
	rand.Seed(time.Now().UnixNano())
	return &Pinger{
		id:      rand.Intn(0xffff),
		seq:     rand.Intn(0xffff),
		address: nil,
		network: "ip",
		source:  "",
		source6: "",
		hasIPv4: false,
		hasIPv6: false,
		Size:    TimeSliceLength,
		MaxRTT:  time.Second,
		OnRecv:  nil,
		OnIdle:  nil,
		Debug:   false,
	}
}

// Network sets a network endpoints for ICMP ping and returns the previous
// setting. network arg should be "ip" or "udp" string or if others are
// specified, it returns an error. If this function isn't called, Pinger
// uses "ip" as default.
func (p *Pinger) Network(network string) (string, error) {
	origNet := p.network
	switch network {
	case "ip":
		fallthrough
	case "udp":
		p.network = network
	default:
		return origNet, errors.New(network + " can't be used as ICMP endpoint")
	}
	return origNet, nil
}

// Source sets ipv4/ipv6 source IP for sending ICMP packets and returns the previous
// setting. Empty value indicates to use system default one (for both ipv4 and ipv6).
func (p *Pinger) Source(source string) (string, error) {
	// using ipv4 previous value for new empty one
	origSource := p.source
	if "" == source {
		p.source = ""
		p.source6 = ""
		return origSource, nil
	}

	addr := net.ParseIP(source)
	if addr == nil {
		return origSource, errors.New(source + " is not a valid textual representation of an IPv4/IPv6 address")
	}

	if isIPv4(addr) {
		p.source = source
	} else if isIPv6(addr) {
		origSource = p.source6
		p.source6 = source
	} else {
		return origSource, errors.New(source + " is not a valid textual representation of an IPv4/IPv6 address")
	}

	return origSource, nil
}

// AddIP adds an IP address to Pinger. ipaddr arg should be a string like
// "192.0.2.1".
func (p *Pinger) AddIP(ipaddr string) error {
	addr := net.ParseIP(ipaddr)
	if addr == nil {
		return fmt.Errorf("%s is not a valid textual representation of an IP address", ipaddr)
	}
	a := &net.IPAddr{IP: addr}
	p.AddIPAddr(a)
	return nil
}

// AddIPAddr adds an IP address to Pinger. ip arg should be a net.IPAddr
// pointer.
func (p *Pinger) AddIPAddr(ip *net.IPAddr) {
	p.address = ip
	if isIPv4(ip.IP) {
		p.hasIPv4 = true
	} else if isIPv6(ip.IP) {
		p.hasIPv6 = true
	}
}

// AddHandler adds event handler to Pinger. event arg should be "receive" or
// "idle" string.
//
// **CAUTION** This function is deprecated. Please use OnRecv and OnIdle field
// of Pinger struct to set following handlers.
//
// "receive" handler should be
//
//	func(addr *net.IPAddr, rtt time.Duration)
//
// type function. The handler is called with a response packet's source address
// and its elapsed time when Pinger receives a response packet.
//
// "idle" handler should be
//
//	func()
//
// type function. The handler is called when MaxRTT time passed. For more
// detail, please see Run() and RunLoop().
func (p *Pinger) AddHandler(event string, handler interface{}) error {
	switch event {
	case "receive":
		if hdl, ok := handler.(func(*net.IPAddr, time.Duration)); ok {
			p.OnRecv = hdl
			return nil
		}
		return errors.New("receive event handler should be `func(*net.IPAddr, time.Duration)`")
	case "idle":
		if hdl, ok := handler.(func()); ok {
			p.OnIdle = hdl
			return nil
		}
		return errors.New("idle event handler should be `func()`")
	}
	return errors.New("No such event: " + event)
}

// Run invokes a single send/receive procedure. It sends packets to all hosts
// which have already been added by AddIP() etc. and wait those responses. When
// it receives a response, it calls "receive" handler registered by AddHander().
// After MaxRTT seconds, it calls "idle" handler and returns to caller with
// an error value. It means it blocks until MaxRTT seconds passed. For the
// purpose of sending/receiving packets over and over, use RunLoop().
func (p *Pinger) Run() error {
	//	p.ctx = newContext()
	p.run()
	//	return p.ctx.err
	return nil
}

func (p *Pinger) listen(netProto string, source string) *icmp.PacketConn {
	conn, err := icmp.ListenPacket(netProto, source)
	if err != nil {
		//		p.ctx.err = err
		p.debugln("Run(): close(p.ctx.done)")
		//		close(p.ctx.done)
		return nil
	}
	return conn
}

func (p *Pinger) run() {
	p.debugln("Run(): Start")
	var conn, conn6 *icmp.PacketConn
	if p.hasIPv4 {
		if conn = p.listen(ipv4Proto[p.network], p.source); conn == nil {
			return
		}
		defer conn.Close()
	}

	if p.hasIPv6 {
		if conn6 = p.listen(ipv6Proto[p.network], p.source6); conn6 == nil {
			return
		}
		defer conn6.Close()
	}

	recv := make(chan *packet, 1)
	//	recvCtx := newContext()
	wg := new(sync.WaitGroup)

	p.debugln("Run(): call recvICMP()")
	if conn != nil {
		wg.Add(1)
		go p.recvICMP(conn, recv, wg)
	}
	if conn6 != nil {
		wg.Add(1)
		go p.recvICMP(conn6, recv, wg)
	}

	p.debugln("Run(): call sendICMP()")
	queue, err := p.sendICMP(conn, conn6)
	if err != nil {
		return
	}

	ticker := time.NewTicker(p.MaxRTT)

mainloop:
	for {
		select {
		// case <-p.ctx.stop:
		// 	p.debugln("Run(): <-p.ctx.stop")
		// 	break mainloop
		// case <-recvCtx.done:
		// 	p.debugln("Run(): <-recvCtx.done")
		// 	err = recvCtx.err
		// 	break mainloop
		case <-ticker.C:
			handler := p.OnIdle
			if handler != nil {
				handler()
			}
			p.Quit = true
			break mainloop
		case r := <-recv:
			p.debugln("Run(): <-recv")
			p.procRecv(r, queue)
		}
	}

	ticker.Stop()

	p.debugln("Run(): close(recvCtx.stop)")
	//	close(recvCtx.stop)
	p.debugln("Run(): wait recvICMP()")
	wg.Wait()

	//	p.ctx.err = err

	p.debugln("Run(): close(p.ctx.done)")
	//	close(p.ctx.done)
	p.debugln("Run(): End")
}

func (p *Pinger) sendICMP(conn, conn6 *icmp.PacketConn) (map[string]*net.IPAddr, error) {
	p.debugln("sendICMP(): Start")
	p.id = rand.Intn(0xffff)
	p.seq = rand.Intn(0xffff)
	queue := make(map[string]*net.IPAddr)
	wg := new(sync.WaitGroup)

	var typ icmp.Type
	var cn *icmp.PacketConn
	if isIPv4(p.address.IP) {
		typ = ipv4.ICMPTypeEcho
		cn = conn
	} else if isIPv6(p.address.IP) {
		typ = ipv6.ICMPTypeEchoRequest
		cn = conn6
	} else {
		return queue, errors.New("bad ip type")
	}
	if cn == nil {
		return queue, errors.New("no connection")
	}

	t := timeToBytes(time.Now())

	if p.Size-TimeSliceLength != 0 {
		t = append(t, byteSliceOfSize(p.Size-TimeSliceLength)...)
	}

	bytes, err := (&icmp.Message{
		Type: typ, Code: 0,
		Body: &icmp.Echo{
			ID: p.id, Seq: p.seq,
			Data: t,
		},
	}).Marshal(nil)
	if err != nil {
		wg.Wait()
		return queue, err
	}

	queue[p.address.String()] = p.address
	var dst net.Addr = p.address
	if p.network == "udp" {
		dst = &net.UDPAddr{IP: p.address.IP, Zone: p.address.Zone}
	}

	p.debugln("sendICMP(): Invoke goroutine")
	wg.Add(1)
	go func(conn *icmp.PacketConn, ra net.Addr, b []byte) {
		for {
			if _, err := conn.WriteTo(bytes, ra); err != nil {
				if neterr, ok := err.(*net.OpError); ok {
					if neterr.Err == syscall.ENOBUFS {
						continue
					}
				}
			}
			break
		}
		p.debugln("sendICMP(): WriteTo End")
		wg.Done()
	}(cn, dst, bytes)
	wg.Wait()
	p.debugln("sendICMP(): End")
	return queue, nil
}

func (p *Pinger) recvICMP(conn *icmp.PacketConn, recv chan<- *packet, wg *sync.WaitGroup) { // , ctx *context
	p.debugln("recvICMP(): Start")
}

func (p *Pinger) procRecv(recv *packet, queue map[string]*net.IPAddr) {
	var ipaddr *net.IPAddr
	switch adr := recv.addr.(type) {
	case *net.IPAddr:
		ipaddr = adr
	case *net.UDPAddr:
		ipaddr = &net.IPAddr{IP: adr.IP, Zone: adr.Zone}
	default:
		return
	}

	addr := ipaddr.String()

	var bytes []byte
	var proto int
	if isIPv4(ipaddr.IP) {
		if p.network == "ip" {
			bytes = ipv4Payload(recv.bytes)
		} else {
			bytes = recv.bytes
		}
		proto = ProtocolICMP
	} else if isIPv6(ipaddr.IP) {
		bytes = recv.bytes
		proto = ProtocolIPv6ICMP
	} else {
		return
	}

	var m *icmp.Message
	var err error
	if m, err = icmp.ParseMessage(proto, bytes); err != nil {
		return
	}

	if m.Type != ipv4.ICMPTypeEchoReply && m.Type != ipv6.ICMPTypeEchoReply {
		return
	}

	var rtt time.Duration
	switch pkt := m.Body.(type) {
	case *icmp.Echo:
		if pkt.ID == p.id && pkt.Seq == p.seq {
			rtt = time.Since(bytesToTime(pkt.Data[:TimeSliceLength]))
		}
	default:
		return
	}

	if _, ok := queue[addr]; ok {
		delete(queue, addr)
		handler := p.OnRecv
		if handler != nil {
			handler(ipaddr, rtt)
		}
	}
}

func (p *Pinger) debugln(args ...interface{}) {
	if p.Debug {
		log.Println(args...)
	}
}

func (p *Pinger) debugf(format string, args ...interface{}) {
	if p.Debug {
		log.Printf(format, args...)
	}
}
