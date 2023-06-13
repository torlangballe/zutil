//go:build !js

package znet

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type forwardProxy struct{}

func StartForwardProxyTunnel(port int) {
	// https://eli.thegreenplace.net/2022/go-and-proxy-servers-part-2-https-proxies/
	addr := fmt.Sprintf(":%d", port)
	log.Println("Starting proxy server on", addr)
	if err := http.ListenAndServe(addr, &forwardProxy{}); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}

func (p *forwardProxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	log.Println("ServeHTTP:", req.Method)
	if req.Method != http.MethodConnect {
		http.Error(w, "this proxy only supports CONNECT", http.StatusMethodNotAllowed)
		return
	}
	log.Printf("CONNECT requested to %v (from %v)", req.Host, req.RemoteAddr)
	targetConn, err := net.Dial("tcp", req.Host)
	if err != nil {
		log.Println("failed to dial to target", req.Host)
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	hj, ok := w.(http.Hijacker)
	if !ok {
		log.Fatal("http server doesn't support hijacking connection")
	}

	clientConn, _, err := hj.Hijack()
	if err != nil {
		log.Fatal("http hijacking failed")
	}

	log.Println("tunnel established")
	go tunnelConn(targetConn, clientConn)
	go tunnelConn(clientConn, targetConn)
}

func tunnelConn(dst io.WriteCloser, src io.ReadCloser) {
	io.Copy(dst, src)
	dst.Close()
	src.Close()
}

