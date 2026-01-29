package xrpc

import (
	"fmt"
	"sync"
	"testing"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znamedfuncs"
	"github.com/torlangballe/zutil/ztesting"
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
	srpc.SetExecutor(executor)
	srpc.SetServer("/", 7743, serverID)
	srpc.AddClient("ws://localhost:7743/", client1ID) // client to self!

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
	crpc.SetExecutor(client2Executor)
	crpc.AddClient("ws://localhost:7743/", client2ID)

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

func TestCircular(t *testing.T) {
	const serverID = "circle-server"
	circleServer = NewRPC()
	executor := znamedfuncs.NewExecutor()
	executor.Register(TestRPCCalls{})
	circleServer.SetExecutor(executor)
	circleServer.SetServer("/", 7744, serverID)
	for i := 0; i <= 100; i++ {
		clientID := makeClientID(i)
		circleServer.AddClient("ws://localhost:7744/", clientID) // client to self!
	}
	var index int = 0
	err := circleServer.Call(makeClientID(0), "TestRPCCalls.PassTheBuck", index, nil)
	wg.Wait()
	ztesting.OnErrorFatal(t, err, "makeFirstClient")
	ztesting.Equal(t, count, 1000, "circular final count")
}
