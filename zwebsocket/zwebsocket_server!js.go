package zwebsocket

import (
	"fmt"
	"net/http"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"golang.org/x/net/websocket"
)

type ClientToServer struct {
	base
}

type Server struct {
	ID                string
	handlerFunc       func(id string, msg []byte, err error) []byte
	Timeout           time.Duration
	Connections       []*ClientToServer
	GotConnectionFunc func(cs *ClientToServer)
}

func NewServer(id, path string, port int, handler func(id string, data []byte, err error) []byte) *Server {
	s := &Server{}
	s.Timeout = time.Second * 10
	s.ID = id
	s.handlerFunc = handler
	router := http.NewServeMux()
	router.Handle(path, websocket.Handler(s.handleSocketRequest))
	addr := fmt.Sprintf(":%d", port)
	go http.ListenAndServe(addr, router)
	return s
}

func (s *Server) addClientToServer(id, path string, conn *websocket.Conn) *ClientToServer {
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

func (s *Server) Exchange(id string, msg []byte) ([]byte, error) {
	zlog.Warn("ServCallingConn:", id, string(msg))
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
	cs := s.addClientToServer(id, req.URL.Path, conn)
	zlog.Info("server connection:", id, req.URL)
	defer conn.Close()
	cs.base.readForever()
}
