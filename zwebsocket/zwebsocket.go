package zwebsocket

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sasha-s/go-deadlock"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type connection struct {
	Connection *websocket.Conn
	Pinger     *ztimer.Repeater
}

type Server struct {
	connections map[string]*connection
	mutex       deadlock.Mutex
	//	mutex       sync.Mutex
}

var upgrader = websocket.Upgrader{} // use default options

func setupConnection(c *connection, server *Server, ping bool, id string) {
	if ping {
		c.Pinger = ztimer.RepeatIn(20, func() bool {
			werr := c.Connection.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*15))
			if werr != nil {
				server.mutex.Lock()
				// let's see if it keeps happening...
				c.Connection.Close()
				delete(server.connections, id)
				server.mutex.Unlock()
				//					got(server, str, nil, true, nil)
				return false
			}
			return true
		})
	}
	server.mutex.Lock()
	server.connections[id] = c
	server.mutex.Unlock()
}

// Init opens a websocket server on a goroutine, on prefix, typically /ws for single one
// got contains:
// n: The server-sendt id sent on connect as a text message, or empty if couldn't make websocket protocol of request
// close: true if connection just closed.
// data: typically json data. nil if this is initial opening call or close.

func handleSocketRequest(w http.ResponseWriter, req *http.Request, ping bool, server *Server, got func(s *Server, id string, data []byte, close bool, err error)) {
	// fmt.Println("ws.handle:", req.RemoteAddr)
	var id string
	c := &connection{}
	conn, err := upgrader.Upgrade(w, req, nil)
	if err != nil {
		got(server, "", nil, false, zlog.Error(err, "upgrade"))
		return
	}
	conn.SetPongHandler(func(string) error {
		// zlog.Info("ws.Pong!")
		conn.SetReadDeadline(time.Now().Add(15))
		return nil
	})
	c.Connection = conn
	defer conn.Close()
	for {
		mt, message, err := conn.ReadMessage()
		if err != nil {
			got(server, id, nil, true, zlog.Error(err, "read message")) // we close on error, it might be only way to exit
			return
		}
		// fmt.Println("ws.read:", mt)
		switch mt {
		case websocket.TextMessage:
			if id == "" {
				id = string(message)
				setupConnection(c, server, ping, id)
				message = nil
			}
			got(server, id, message, false, nil)
			// fmt.Println("ws.text done:", mt, id)
		case websocket.CloseMessage:
			fmt.Println("ws.disconnect:", conn)
			server.mutex.Lock()
			if c.Pinger != nil {
				c.Pinger.Stop()
			}
			delete(server.connections, id)
			server.mutex.Unlock()
			got(server, id, nil, true, nil)
		case websocket.BinaryMessage:
			// fmt.Println("ws.binary:", conn)
			_, r, err := conn.NextReader()
			if err != nil {
				got(server, id, nil, false, zlog.Error(err, "next reader"))
			}
			data, err := ioutil.ReadAll(r)
			if err != nil {
				got(server, id, nil, false, zlog.Error(err, "read all"))
			}
			got(server, id, data, false, nil)
		}
	}
}

func NewServer(prefix string, port int, ping bool, got func(s *Server, id string, data []byte, close bool, err error)) *Server { // "/ws"
	server := &Server{}
	server.connections = map[string]*connection{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	http.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		handleSocketRequest(w, r, ping, server, got)
	})
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			got(server, "", nil, false, zlog.Error(err, "listen"))
		}
	}()
	return server
}

func (c *Server) findConnection(wc *websocket.Conn) (string, *connection) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for str, con := range c.connections {
		if wc == con.Connection {
			return str, con
		}
	}
	return "", nil
}

// Close closes the connection to server 'id', and removes it from the map
func (c *Server) Close(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	con, got := c.connections[id]
	if !got {
		return zlog.Error(nil, "no connection to close:", id)
	}
	err := con.Connection.Close()
	delete(c.connections, id)
	return err
}

func (c *Server) Send(id string, structure interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	con, got := c.connections[id]
	if !got {
		return zlog.Error(nil, "no connection to send to:", id)
	}
	con.Connection.SetWriteDeadline(time.Now().Add(time.Second * 15))
	return con.Connection.WriteJSON(structure)
}

func (c *Server) IDs() []string {
	c.mutex.Lock()
	ids := make([]string, len(c.connections), len(c.connections))
	i := 0
	for id := range c.connections {
		ids[i] = id
		i++
	}
	c.mutex.Unlock()
	return ids
}

type Client struct {
	Connection *websocket.Conn
}

func ClientNew(address string, receive func(message []byte, err error) bool) (*Client, error) {
	var err error
	c, _, err := websocket.DefaultDialer.Dial(address, nil)
	if err != nil {
		return nil, zlog.Error(err, "dial")
	}
	client := &Client{Connection: c}
	defer c.Close()

	// done := make(chan struct{})

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				zlog.Error(err, "read")
			}
			if receive != nil {
				if !receive(message, err) {
					return
				}
			}
			zlog.Info("receive wesocket:", message)
		}
	}()
	return client, nil
}

func (c *Client) Send(structure interface{}) error {
	return c.Connection.WriteJSON(structure)
}
