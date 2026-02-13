package xrpc

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/ztesting"
	"github.com/torlangballe/zutil/ztimer"
	"github.com/torlangballe/zutil/zwebsocket"
)

type TestRPCCalls struct{}
type ClientTestRPCCalls struct{}

type Two struct {
	A int
	B int
}

type One struct {
	Sum int
}

func (TestRPCCalls) Product(arg Two, result *One) error {
	result.Sum = arg.A * arg.B
	return nil
}

func (ClientTestRPCCalls) Product(arg Two, result *One) error {
	result.Sum = arg.A + arg.B
	return nil
}

func xTestSendReceive(t *testing.T) {
	const serverID = "testrpcserver"
	const client1ID = "client1"
	const client2ID = "client2"

	srpc := NewRPC()
	executor := znamedfuncs.NewExecutor()
	executor.Register(TestRPCCalls{})
	srpc.Executor = executor
	srpc.ConnectServerFunc = func(serverID string) (*zwebsocket.Server, error) {
		return srpc.MakeServer("/", 7743)
	}
	srpc.ConnectClientFunc = func(clientID string) (*zwebsocket.Client, error) {
		addr := "ws://localhost:7743/"
		client, err := srpc.MakeClient(addr, clientID)
		return client, err
	}
	client2Executor := znamedfuncs.NewExecutor()
	client2Executor.Register(ClientTestRPCCalls{})

	var result One
	err := srpc.Call(serverID, "TestRPCCalls.Product", Two{A: 3, B: 4}, &result)
	if err != nil {
		t.Error("RPC call error:", err)
	}
	ztesting.Equal(t, result.Sum, 12, "call1 not 3*4")
	err = srpc.Call(client1ID, "TestRPCCalls.Product", Two{A: 12, B: 12}, &result)
	if err != nil {
		t.Error("RPC call error2:", err)
	}
	ztesting.Equal(t, result.Sum, 144, "call2 not 12*12")

	crpc := NewRPC()
	crpc.Executor = client2Executor
	crpc.ConnectClientFunc = func(clientID string) (*zwebsocket.Client, error) {
		addr := "ws://localhost:7743/"
		client, err := crpc.MakeClient(addr, clientID)
		return client, err
	}
	err = crpc.Call(client2ID, "TestRPCCalls.Product", Two{A: 9, B: 4}, &result)
	if err != nil {
		t.Error("RPC call error3:", err)
	}
	ztesting.Equal(t, result.Sum, 36, "call3 not 9*4")

	err = srpc.Call(client2ID, "ClientTestRPCCalls.Product", Two{A: 5, B: 5}, &result)
	if err != nil {
		t.Fatal("RPC call error4:", err)
	}
	ztesting.Equal(t, result.Sum, 10, "should be A+B cause it's client2")

	err = crpc.Call("", "TestRPCCalls.Product", Two{A: 7, B: 6}, &result)
	if err != nil {
		t.Fatal("RPC call error5:", err)
	}
	ztesting.Equal(t, result.Sum, 42, "should be 7*6 cause it's client1 calling server")
}

func makeClientID(i int) string {
	return fmt.Sprintf("clientcirclejerk%d", i)
}

const circleLength = 100

var circleServer *RPC
var count int
var wg sync.WaitGroup

func (TestRPCCalls) PassTheBuck(index int) error {
	if count >= 1000 {
		return nil
	}
	wg.Add(1)
	count++
	if index == circleLength {
		index = 0
	}
	// zlog.Warn("Passing the buck at index:", index, "count:", count)
	go func(index int) {
		err := circleServer.Call(makeClientID(index+1), "TestRPCCalls.PassTheBuck", index+1, nil)
		zlog.OnError(err, "PassTheBuck call")
		wg.Done()
	}(index)
	return nil
}

func xTestCircular(t *testing.T) {
	const serverID = "circle-server"
	circleServer = NewRPC()
	executor := znamedfuncs.NewExecutor()
	executor.Register(TestRPCCalls{})
	circleServer.Executor = executor
	circleServer.ConnectServerFunc = func(serverID string) (*zwebsocket.Server, error) {
		return circleServer.MakeServer("/", 7744)
	}
	circleServer.AddServer(serverID)
	circleServer.ConnectClientFunc = func(clientID string) (*zwebsocket.Client, error) {
		addr := "ws://localhost:7744/"
		client, err := circleServer.MakeClient(addr, clientID)
		return client, err
	}
	for i := 0; i <= 100; i++ {
		clientID := makeClientID(i)
		circleServer.AddClient(clientID)
	}
	var index int = 0
	err := circleServer.Call(makeClientID(0), "TestRPCCalls.PassTheBuck", index, nil)
	wg.Wait()
	ztesting.OnErrorFatal(t, err, "makeFirstClient")
	ztesting.Equal(t, count, 1000, "circular final count")
}

func makeRPC(t *testing.T, port int, executor *znamedfuncs.Executor) {
	serverRPC := NewRPC()
	serverRPC.Executor = executor
	serverRPC.ConnectServerFunc = func(serverID string) (*zwebsocket.Server, error) {
		return serverRPC.MakeServer("/", port)
	}
	serverID := fmt.Sprintf("server%d", port)
	serverRPC.AddServer(serverID)
	var err error
	clientRPC := NewRPC()
	clientRPC.Executor = executor
	clientRPC.ConnectClientFunc = func(clientID string) (*zwebsocket.Client, error) {
		addr := fmt.Sprintf("ws://localhost:%d/", port)
		return clientRPC.MakeClient(addr, clientID)
	}
	clientID := fmt.Sprintf("client%d", port)
	clientRPC.AddClient(clientID)
	if err != nil {
		t.Fatal(err)
	}
	var serverSend, clientSend int
	i := 0
	now := time.Now()
	clientRPC.Start()
	serverRPC.Start()
	ztimer.Repeat(0.001, func() bool {
		var product One
		a := rand.IntN(200)
		b := rand.IntN(200)
		err := serverRPC.Call(clientID, "TestRPCCalls.Product", Two{A: a, B: b}, &product) // calls TO client
		if err == nil {
			serverSend++
			ztesting.Equal(t, product.Sum, a*b, "product should be a*b")
			time.Sleep(time.Millisecond * time.Duration(1+rand.IntN(5)))
		}
		a = rand.IntN(200)
		b = rand.IntN(200)
		err = clientRPC.Call(clientID, "TestRPCCalls.Product", Two{A: a, B: b}, &product) // calls FROM client
		// zlog.Warn("ClientCall", err)
		if err != nil {
			return time.Since(now) < time.Second*2
		} else {
			clientSend++
		}
		if rand.IntN(50) == 20 {
			for _, s := range serverRPC.servers {
				if s.connection == nil {
					continue
				}
				zlog.Warn("Storm", port, "closing server")
				s.connection.Close()
				s.connection = nil
			}
		}
		if rand.IntN(50) == 30 {
			zlog.Warn("Storm", port, "closing client")
			client := clientRPC.ClientForID(clientID)
			if client != nil {
				client.Close()
			}
		}
		ztesting.Equal(t, product.Sum, a*b, "product should be a*b 2", a, b)
		time.Sleep(time.Millisecond * time.Duration(rand.IntN(5)))
		i++
		// zlog.Warn("Ongoing:", serverSend, clientSend)
		return time.Since(now) < time.Second*2
	})
	time.Sleep(time.Millisecond * 2100)
	zlog.Warn("Storm", port, "server sent:", serverSend, "client sent:", clientSend)
}

func TestFaultyStorm(t *testing.T) {
	const serverID = "faulty-storm-server"
	executor := znamedfuncs.NewExecutor()
	executor.Register(TestRPCCalls{})
	makeRPC(t, 7744, executor)
	time.Sleep(time.Second * 2)
}
