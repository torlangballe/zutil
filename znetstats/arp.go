//go:build server

package znetstats

import (
	"context"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/torlangballe/zutil/zlog"
)

func (n *NetStats) listenARP(ctx context.Context) {
	handle, err := pcap.OpenLive(n.iface, 1024, false, 10*time.Second)
	if err != nil {
		zlog.Fatal("pcap:", err)
	}
	defer handle.Close()
	handle.SetBPFFilter("arp")
	ps := gopacket.NewPacketSource(handle, handle.LinkType())
	for {
		select {
		case <-ctx.Done():
			return
		case p := <-ps.Packets():
			arp := p.Layer(layers.LayerTypeARP).(*layers.ARP)
			if arp.Operation == 2 {
				mac := net.HardwareAddr(arp.SourceHwAddress)
				m := n.lookupManuf(mac.String())
				n.pushData(ParseIP(arp.SourceProtAddress).String(), mac, "", m)
				parsed := ParseIP(arp.SourceProtAddress)
				go func() {
					n.sendMdns(parsed, mac)
					n.sendNbns(parsed, mac)
				}()
			}
		}
	}
}

func (n *NetStats) sendArpPackage(ip IP) {
	srcIp := net.ParseIP(n.ipNet.IP.String()).To4()
	dstIp := net.ParseIP(ip.String()).To4()
	if srcIp == nil || dstIp == nil {
		zlog.Fatal("ip 解析出问题")
	}
	// 以太网首部
	// EthernetType 0x0806  ARP
	ether := &layers.Ethernet{
		SrcMAC:       n.localHaddr,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}

	a := &layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     uint8(6),
		ProtAddressSize:   uint8(4),
		Operation:         uint16(1), // 0x0001 arp request 0x0002 arp response
		SourceHwAddress:   n.localHaddr,
		SourceProtAddress: srcIp,
		DstHwAddress:      net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		DstProtAddress:    dstIp,
	}

	buffer := gopacket.NewSerializeBuffer()
	var opt gopacket.SerializeOptions
	gopacket.SerializeLayers(buffer, opt, ether, a)
	outgoingPacket := buffer.Bytes()

	handle, err := pcap.OpenLive(n.iface, 2048, false, 30*time.Second)
	if err != nil {
		zlog.Fatal("pcap:", err)
	}
	defer handle.Close()

	err = handle.WritePacketData(outgoingPacket)
	if err != nil {
		zlog.Fatal("发送arp数据包失败..")
	}
}
