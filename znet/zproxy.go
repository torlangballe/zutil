//go:build !js

package znet

/*

type forwardProxy struct {
	ws *zwebsocket.Server
}

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

*/

/*
func StartOutsideTunnel(firewalledServerConnectionPort, portToTunnel int) {
	var session *yamux.Session
	var mu sync.Mutex

	// 1. Listen for the firewalled client (Control connection)
	go func() {
		controlListener, err := net.Listen("tcp", fmt.Sprintf(":%d", firewalledServerConnectionPort))
		if err != nil {
			log.Fatalf("Control listener error: %v", err)
		}
		log.Printf("Waiting for firewalled machine on :%d...", firewalledServerConnectionPort)

		for {
			conn, err := controlListener.Accept()
			if err != nil {
				continue
			}
			// Initialize Yamux client session over the connection
			sess, err := yamux.Client(conn, nil)
			if err != nil {
				conn.Close()
				continue
			}
			mu.Lock()
			session = sess
			mu.Unlock()
			log.Println("Firewalled machine connected. Tunnel ready.")
		}
	}()

	// 2. Intercept calls on the public target port (e.g., :9090)
	publicListener, err := net.Listen("tcp", fmt.Sprintf(":%d", portToTunnel))
	if err != nil {
		log.Fatalf("Public intercept listener error: %v", err)
	}
	log.Printf("Intercepting traffic on port :%d...", portToTunnel)

	for {
		incomingConn, err := publicListener.Accept()
		if err != nil {
			continue
		}

		go func(src net.Conn) {
			defer src.Close()

			mu.Lock()
			sess := session
			mu.Unlock()

			if sess == nil || sess.IsClosed() {
				log.Println("Error: No firewalled machine connected to tunnel.")
				return
			}

			// Open a virtual stream through Yamux to the firewalled machine
			tunnelStream, err := sess.Open()
			if err != nil {
				log.Printf("Failed to open Yamux stream: %v", err)
				return
			}
			defer tunnelStream.Close()

			// Bidirectionally pipe the raw intercepted data
			errChan := make(chan error, 2)
			go func() {
				_, err := io.Copy(tunnelStream, src)
				errChan <- err
			}()
			go func() {
				_, err := io.Copy(src, tunnelStream)
				errChan <- err
			}()

			<-errChan // Wait until one side finishes or fails
		}(incomingConn)
	}
}

func StartTunnelOnFirewalledMachine(outsideServerAddress string) error {
	// 1. Establish the connection to the public server
	conn, err := net.Dial("tcp", outsideServerAddress)
	if err != nil {
		return fmt.Errorf("could not connect to public server: %v", err)
	}
	zlog.Info("Connected to public server. Starting Yamux multiplexer...")

	// 2. Host the Yamux server session over this socket
	session, err := yamux.Server(conn, nil)
	if err != nil {
		return err
	}

	// 3. Accept incoming streams (intercepted connections) forwarded by the server
	for {
		stream, err := session.Accept()
		if err != nil {
			return fmt.Errorf("Session closed or stream error: %v", err)
		}

		go func(tunnelStream net.Conn) {
			defer tunnelStream.Close()

			// 4. Re-call the request locally (or forward to another target behind the firewall)
			// e.g., a local web server running on port 3000
			destinationConn, err := net.Dial("tcp", "127.0.0.1:80")
			if err != nil {
				zlog.Error("Failed to connect to local target: %v", err)
				return
			}
			defer destinationConn.Close()

			// Pipe data between the tunnel stream and the actual application
			errChan := make(chan error, 2)
			go func() {
				_, err := io.Copy(destinationConn, tunnelStream)
				errChan <- err
			}()
			go func() {
				_, err := io.Copy(tunnelStream, destinationConn)
				errChan <- err
			}()

			<-errChan
		}(stream)
	}
	return fmt.Errorf("Tunnel session ended")
}
*/
