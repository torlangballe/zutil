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
	Ponger     *ztimer.Repeater
}

type Client struct {
	connections map[string]*connection
	mutex       sync.Mutex
}

var upgrader = websocket.Upgrader{} // use default options

// Init opens a websocket server on a goroutine,on prefix, typically /ws for single one
// got contains:
// n: The client-sendt id sent on connect as a text message, or empty if couldn't make websocket protocol of request
// close: true if connection just closed.
// data: typically json data. nil if this is initial opening call or close.

func Init(prefix string, port int, got func(c *Client, id string, data []byte, close bool, err error)) *Client { // "/ws"
	client := &Client{}
	client.connections = map[string]*connection{}
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	http.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			got(client, "", nil, false, zlog.Error(err, "upgrade"))
			return
		}
		mt, message, err := conn.ReadMessage()
		if err != nil {
			got(client, "", nil, false, zlog.Error(err, "read message"))
			return
		}
		switch mt {
		case websocket.TextMessage:
			str := string(message)
			c := &connection{}
			c.Connection = conn
			c.Ponger = ztimer.RepeatIn(20, func() bool {
				// fmt.Println("ws.ping:", str)
				werr := conn.WriteControl(websocket.PongMessage, nil, time.Now().Add(time.Second*15))
				if werr != nil {
					fmt.Println("ws.ping failed, closing:", str)
					client.mutex.Lock()
					// let's see if it keeps happening...
					// conn.Close()
					// delete(client.connections, str)
					client.mutex.Unlock()
					//					got(client, str, nil, true, nil)
					return false
				}
				return true
			})
			client.mutex.Lock()
			client.connections[str] = c
			client.mutex.Unlock()
			got(client, str, nil, false, nil)
			// fmt.Println("ws.connect:", mt, str)
		case websocket.CloseMessage:
			// fmt.Println("ws.disconnect:", conn)
			str, c1 := client.findConnection(conn)
			if c1 != nil {
				client.mutex.Lock()
				c1.Ponger.Stop()
				delete(client.connections, str)
				client.mutex.Unlock()
				got(client, str, nil, true, nil)
			} else {
				got(client, "", nil, true, zlog.Error(nil, "closed connection not found"))
			}
		case websocket.BinaryMessage:
			// fmt.Println("ws.binary:", conn)
			str, c1 := client.findConnection(conn)
			if c1 != nil {
				_, r, err := conn.NextReader()
				if err != nil {
					got(client, str, nil, false, zlog.Error(err, "next reader"))
				}
				data, err := ioutil.ReadAll(r)
				if err != nil {
					got(client, str, nil, false, zlog.Error(err, "read all"))
				}
				got(client, str, data, false, nil)
			} else {
				got(client, "", nil, true, zlog.Error(nil, "binary message: connection not found"))
			}
		}
	})
	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			got(client, "", nil, false, zlog.Error(err, "listen"))
		}
	}()
	return client
}

func (c *Client) findConnection(wc *websocket.Conn) (string, *connection) {
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

// Close closes the connection to client 'id', and removes it from the map
func (c *Client) Close(id string) error {
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

func (c *Client) Send(id string, structure interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	con, got := c.connections[id]
	if !got {
		return zlog.Error(nil, "no connection to send to:", id)
	}
	return con.Connection.WriteJSON(structure)
}

func (c *Client) IDs() []string {
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
