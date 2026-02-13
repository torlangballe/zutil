package zwebsocket

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zslice"
	"golang.org/x/net/websocket"
)

type ClientToServer struct {
	base
}

type Server struct {
	handlerFunc       func(id string, msg []byte, err error) []byte
	Timeout           time.Duration
	Connections       []*ClientToServer
	GotConnectionFunc func(cs *ClientToServer)
	httpServer        *http.Server
}

func NewServer(path string, port int, handler func(id string, data []byte, err error) []byte) (*Server, error) {
	s := &Server{}
	s.Timeout = time.Second * 10
	s.handlerFunc = handler
	router := http.NewServeMux()
	router.Handle(path, websocket.Handler(s.handleSocketRequest))
	addr := fmt.Sprintf(":%d", port)
	s.httpServer = &http.Server{Addr: addr, Handler: router}
	var err error
	go func() {
		err = s.httpServer.ListenAndServe()
		// zlog.Warn("AfterListen")
	}()
	time.Sleep(time.Millisecond * 50) // Give ListenAndServe time to start
	return s, err
}

func (s *Server) Close() {
	for _, c := range s.Connections {
		if c.conn != nil {
			c.conn.Close()
		}
	}
	s.Connections = []*ClientToServer{}
	s.httpServer.Close()
}

func (s *Server) RemoveConnection(id string) {
	for i, c := range s.Connections {
		if c.ID == id {
			if c.conn != nil {
				c.conn.Close()
			}
			zslice.RemoveAt(&s.Connections, i)
			return
		}
	}
}

func (s *Server) setClientToServer(id, path string, conn *websocket.Conn) *ClientToServer {
	s.Connections = slices.DeleteFunc(s.Connections, func(c *ClientToServer) bool {
		return c.ID == id
	})
	cs := &ClientToServer{}
	cs.ID = id
	cs.handlerFunc = func(msg []byte, err error) []byte {
		return s.handlerFunc(cs.ID, msg, err)
	}
	cs.Timeout = s.Timeout
	cs.conn = conn
	cs.url = path
	s.Connections = append(s.Connections, cs)
	if s.GotConnectionFunc != nil {
		go s.GotConnectionFunc(cs)
	}
	return cs
}

func (s *Server) Exchange(msg []byte) ([]byte, error) {
	if len(s.Connections) != 1 {
		return nil, fmt.Errorf("not 1 connection only")
	}
	return s.Connections[0].Exchange(msg)
}

func (s *Server) ExchangeWithID(id string, msg []byte) ([]byte, error) {
	// zlog.Warn("Server ExchangeWithID", id, s != nil)
	for _, c := range s.Connections {
		if c.ID == id {
			return c.Exchange(msg)
		}
	}
	return nil, fmt.Errorf("no such connection id:%s", id)
}

func (s *Server) handleSocketRequest(conn *websocket.Conn) {
	req := conn.Request()
	id := req.Header.Get(IDHeader)
	cs := s.setClientToServer(id, req.URL.Path, conn)
	zlog.Info("server connection:", id, req.URL)
	defer conn.Close()
	cs.base.readForever()
}
