package umulticast

import (
	"errors"
	"fmt"
	"net"
	"time"
)

// Listen binds to the UDP address and port given and writes packets received
// from that address to a buffer which is passed to a hander
func Listen(addr *net.UDPAddr, timeoutSecs float64, maxDatagramSize int, handler func(*net.UDPAddr, int, []byte, error)) (conn *net.UDPConn) {
	var err error
	conn, err = net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		fmt.Println("umulticast.Listen ListenMulticastUDP err:", err)
		handler(nil, 0, nil, err)
		return
	}
	conn.SetReadBuffer(maxDatagramSize)
	timer := time.NewTimer(time.Duration(float64(time.Second) * timeoutSecs))
	var gerr error
	go func() {
		<-timer.C
		//		fmt.Println("umlisten.Listen timeout:", addr, timeoutSecs)
		gerr = errors.New("timeout umlisten")
		conn.Close()
	}()
	go func() {
		for {
			buffer := make([]byte, maxDatagramSize)
			numBytes, src, e := conn.ReadFromUDP(buffer)
			timer.Stop()
			if e != nil {
				conn.Close()
				if gerr != nil {
					e = gerr
				}
				handler(src, 0, nil, e)
				return
			}
			buffer = buffer[:numBytes]
			handler(src, numBytes, buffer, nil)
		}
	}()
	return
}
