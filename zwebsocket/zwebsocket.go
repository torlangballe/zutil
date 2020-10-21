package zwebsocket

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

type connection struct {
	Connection *websocket.Conn
	Pinger     *ztimer.Repeater
}

type Server struct {
	connections map[string]*connection
	mutex       sync.Mutex
}

var upgrader = websocket.Upgrader{} // use default options

// Init opens a websocket server on a goroutine, on prefix, typically /ws for single one
// got contains:
// n: The server-sendt id sent on connect as a text message, or empty if couldn't make websocket protocol of request
// close: true if connection just closed.
// data: typically json data. nil if this is initial opening call or close.

func NewServer(prefix string, port int, ping bool, got func(s *Server, id string, data []byte, close bool, err error)) *Server { // "/ws"
	server := &Server{}
	server.connections = map[string]*connection{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	http.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			got(server, "", nil, false, zlog.Error(err, "upgrade"))
			return
		}
		conn.SetPongHandler(func(string) error {
			zlog.Info("ws.Pong!")
			conn.SetReadDeadline(time.Now().Add(15))
			return nil
		})
		mt, message, err := conn.ReadMessage()
		if err != nil {
			got(server, "", nil, false, zlog.Error(err, "read message"))
			return
		}
		switch mt {
		case websocket.TextMessage:
			str := string(message)
			c := &connection{}
			c.Connection = conn
			if ping {
				c.Pinger = ztimer.RepeatIn(20, func() bool {
					werr := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second*15))
					if werr != nil {
						server.mutex.Lock()
						// let's see if it keeps happening...
						conn.Close()
						delete(server.connections, str)
						server.mutex.Unlock()
						//					got(server, str, nil, true, nil)
						return false
					}
					return true
				})
			}
			server.mutex.Lock()
			server.connections[str] = c
			server.mutex.Unlock()
			got(server, str, nil, false, nil)
			// fmt.Println("ws.connect:", mt, str)
		case websocket.CloseMessage:
			fmt.Println("ws.disconnect:", conn)
			str, c1 := server.findConnection(conn)
			if c1 != nil {
				server.mutex.Lock()
				if c1.Pinger != nil {
					c1.Pinger.Stop()
				}
				delete(server.connections, str)
				server.mutex.Unlock()
				got(server, str, nil, true, nil)
			} else {
				got(server, "", nil, true, zlog.Error(nil, "closed connection not found"))
			}
		case websocket.BinaryMessage:
			// fmt.Println("ws.binary:", conn)
			str, c1 := server.findConnection(conn)
			if c1 != nil {
				_, r, err := conn.NextReader()
				if err != nil {
					got(server, str, nil, false, zlog.Error(err, "next reader"))
				}
				data, err := ioutil.ReadAll(r)
				if err != nil {
					got(server, str, nil, false, zlog.Error(err, "read all"))
				}
				got(server, str, data, false, nil)
			} else {
				got(server, "", nil, true, zlog.Error(nil, "binary message: connection not found"))
			}
		}
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
	for str, con := range c.connections {
		if wc == con.Connection {
			c.mutex.Unlock()
			return str, con
		}
	}
	c.mutex.Unlock()
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
