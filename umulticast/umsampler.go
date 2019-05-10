package umulticast

import (
	"btech/mdc/tspacket"
	"fmt"
	"net"
	"time"
)

type MulticastReader struct {
	multiConn    *net.UDPConn
	bytesRead    int
	secTicker    *time.Ticker
	connectStart time.Time
	multiAddress net.UDPAddr
	stopped      bool

	HandleMulticastConnected func(source, dest net.UDPAddr, joinlat time.Duration)
	HandleMulticastBPS       func(bytesPerSecond int64)
	HandleMulticastError     func(err error)
	HandlePackage            func(data []byte)
}

func NewMulticastReader(address net.IP, port int, firstReadTimeoutSecs float64) (reader *MulticastReader) {
	reader = &MulticastReader{}
	reader.connectStart = time.Now()
	reader.multiAddress = net.UDPAddr{IP: address, Zone: "", Port: port}
	fmt.Printf("NewMulticastReader1 %v %v %v %p\n", address, port, reader.multiAddress, reader)

	first := true
	reader.multiConn = Listen(&reader.multiAddress, firstReadTimeoutSecs, 8192, func(source *net.UDPAddr, length int, data []byte, err error) {
		if err != nil {
			if !reader.stopped {
				reader.HandleMulticastError(err)
			}
			return
		}
		if reader.HandlePackage != nil {
			if tspacket.Packet(data).IsValid() { // for now we just check if it is a valid TSPacket. So RTP packets don't mess everything up
				go reader.HandlePackage(data)
			}
		}
		if reader.secTicker == nil {
			if !first {
				panic("umsampler.NewMulticastReader called before first=false")
			}
			first = false
			joinlat := time.Since(reader.connectStart)
			reader.HandleMulticastConnected(*source, reader.multiAddress, joinlat)
			reader.secTicker = time.NewTicker(time.Second) // do this after handler above for more accurat 1 sec later
			go func() {
				for range reader.secTicker.C {
					//					fmt.Printf("reader.secTicker: %p\n", reader)
					if reader.HandleMulticastBPS != nil {
						reader.HandleMulticastBPS(int64(reader.bytesRead))
					}
					//					fmt.Println("reader.secTicker2")
					reader.bytesRead = 0
				}
			}()
		}
		reader.bytesRead += len(data)
		//		fmt.Println("got:", source, length, length, hex.EncodeToString(data[:4]))
	})
	return
}

func (reader *MulticastReader) Stop() {
	//	fmt.Printf("MulticastReader.Stop: %p\n", reader)
	reader.stopped = true
	if reader.secTicker != nil {
		reader.secTicker.Stop()
	}
	if reader.multiConn != nil {
		reader.multiConn.Close()
	}
}
