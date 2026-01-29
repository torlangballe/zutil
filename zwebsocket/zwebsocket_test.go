//go:build !js

package zwebsocket

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zui/zapp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/ztesting"
)

type Exchanger interface {
	Exchange(msg []byte, reply *[]byte) error
	ExchangeFunc(msg []byte, got func([]byte, error))
}

const (
	port = 8781
)

var clientSends, serverSends, clientReceives, serverReceives int

func openClient(t *testing.T) *Client {
	client, err := NewClient("ws://localhost:7781/", func(data []byte, err error) []byte {
		str := string(data)
		// zlog.Warn("Client Got:", string(data))
		if !strings.HasPrefix(str, "server ") {
			t.Error("Client received unexpected string:", str)
			return nil
		}
		clientReceives++
		return []byte(str + " reply")
	})
	if err != nil {
		t.Error("Failed to open client:", err)
		return nil
	}
	return client
}

func openServer(t *testing.T) *Server {
	server := NewServer("/", 7781, func(data []byte, err error) []byte {
		str := string(data)
		// zlog.Warn("Server Got:", str)
		serverReceives++
		if !strings.HasPrefix(str, "client ") {
			t.Error("Server received unexpected string:", str)
			return nil
		}
		return []byte(str + " reply")
	})
	return server
}

func doRandomFires(t *testing.T, e Exchanger, pre string, add *int) {
	for _ = range 20000 {
		var reply []byte
		(*add)++
		str := fmt.Sprintf("%s %d", pre, rand.Int63())
		err := e.Exchange([]byte(str), &reply)
		if err != nil {
			t.Error("Exchange error:", err)
		}
		ztesting.Equal(t, string(reply), str+" reply")
	}
}

func doRandomFiresWithFunc(t *testing.T, e Exchanger, pre string, add *int) {
	for _ = range 20000 {
		str := fmt.Sprintf("%s %d", pre, rand.Int63())
		e.ExchangeFunc([]byte(str), func(reply []byte, err error) {
			(*add)++
			if err != nil {
				t.Error("Exchange error:", err)
			}
			ztesting.Equal(t, string(reply), str+" reply")
		})
	}
}

func testReadWrite(t *testing.T, server *Server, client *Client) {
	zlog.Warn("testReadWrite")
	go doRandomFiresWithFunc(t, client, "client", &clientSends)
	time.Sleep(time.Second * 5)
	m := map[string]int{
		"clientSends":    clientSends,
		"serverReceives": serverReceives,
		"serverSends":    serverSends,
		"clientReceives": clientReceives,
	}
	for s, c := range m {
		ztesting.GreaterThan(t, c, 19000, "Not enough ", s, "received")
	}
	zlog.Warn("Result:", zlog.Full(m))
}

func TestAll(t *testing.T) {
	router := mux.NewRouter()
	zapp.Init(nil)
	zapp.ServeZUIWasm(router, false, nil)
	addr := fmt.Sprintf(":%d", port)
	znet.ServeHTTPInBackground(addr, "", router)
	server := openServer(t)
	server.GotConnectionFunc = func(cs *ClientToServer) {
		zlog.Warn("Server got connection from", cs.url)
		// doRandomFiresWithFunc(t, cs, "server", &serverSends)
	}
	client := openClient(t)
	testReadWrite(t, server, client)
}
