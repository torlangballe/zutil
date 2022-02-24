package zwrpc

import (
	"errors"
	"testing"
	"time"
)

type TestCalls struct{}

const (
	port      = 1344
	ipAddress = "127.0.0.1"
)

var TCalls = new(TestCalls)

func (c *TestCalls) TestHelloWorld(arg string, result *string) error {
	if arg != "Hello" {
		return errors.New("Wrong message: " + arg)
	} else {
		*result = "Hi"
	}
	return nil
}

func TestAll(t *testing.T) {
	InitServer("/Users/tor/source/go/src/github.com/torlangballe/etheros/certs/etheros", port)
	Register(TCalls)
	time.Sleep(time.Millisecond * 100) // to make sure InitServer started serving
	testSimpleCall(t)
	time.Sleep(time.Millisecond * 200) // to make sure InitServer started serving
	testReceiveCall(t)
}

func testSimpleCall(t *testing.T) {
	client, err := NewClient(ipAddress, port, "")
	if err != nil {
		t.Error(err, "new client")
		return
	}
	defer client.Close()
	var result string
	err = client.Call("TestCalls.TestHelloWorld", "Hello", &result)
	if err != nil {
		t.Error(err, "call")
		return
	}
	if result != "Hi" {
		t.Error(err, "compare", result)
		return
	}
}

func testReceiveCall(t *testing.T) {
	send, _, err := NewSendAndReceiveClients(ipAddress, "client-server", port)
	if err != nil {
		t.Error(err, "newSendAndReceiveClient")
		return
	}
	var result string
	err = send.Call("TestCalls.TestHelloWorld", "Hello", &result)
	if result != "Hi" {
		t.Error(err, "compare", result)
		return
	}
}
