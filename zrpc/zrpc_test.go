//go:build server

package zrpc

import (
	"fmt"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
)

type CloudCall struct{}
type WebCall struct{}

const port = 95

var (
	webClient   *Client
	cloudRC     *ReverseClient
	revClienter *ReverseClientsOwner
)

func (CloudCall) HelloTo5(str string, result *int) error {
	zlog.Warn("Hello:", str)
	*result = 5
	return nil
}

func (WebCall) HelloTo7(str string, result *int) error {
	zlog.Warn("Hello:", str)
	*result = 7
	return nil
}

func (WebCall) Add(in int, result *int) error {
	*result = in + 1
	return nil
}

func (WebCall) Color(i int, result *string) error {
	cols := []string{"red", "green", "blue", "black", "yellow", "orange", "purple", "magenta", "cyan", "white"}
	if i < 0 || i > 9 {
		return zlog.NewError(nil, "i outside", i)
	}
	zlog.Warn("Color:", i, *result)
	*result = cols[i]
	return nil
}

func testCall(t *testing.T) {
	zlog.Warn("testCall")
	var result int
	err := webClient.Call("CloudCall.HelloTo5", "bonjour", &result)
	if err != nil {
		t.Error(err)
		return
	}
	if result != 5 {
		t.Error("result is not 5:", result)
		return
	}
}

func testReverseCall(t *testing.T) {
	zlog.Warn("testReverseCall")
	var result int
	err := cloudRC.Call("WebCall.HelloTo7", "heisan", &result)
	if err != nil {
		t.Error(err)
		return
	}
	if result != 7 {
		t.Error("result is not 7:", result)
		return
	}
}

func testReverseAdd(t *testing.T) {
	zlog.Warn("testReverseAdd")
	var count int
	for i := 0; i < 10; i++ {
		err := cloudRC.Call("WebCall.Add", count, &count)
		if err != nil {
			t.Error(err)
			return
		}
	}
	if count != 10 {
		t.Error("count is not 10:", count)
		return
	}
}

func testMultiWeb(t *testing.T) {
	surl := fmt.Sprint("http://localhost:", port)
	for i := 0; i < 10; i++ {
		client := NewClient(surl, fmt.Sprint("web-", i+1))
		exe := NewExecutor()
		NewReverseExecutor(client, "", exe)
		exe.Register(WebCall{})
	}
	time.Sleep(time.Millisecond * 400)
	var getter RowGetter = func(receiverID string, index int) any {
		return index
	}
	ms := ReverseCallAll[string](revClienter, 5, "WebCall.Color", "web-*", getter)
	for _, m := range ms {
		zlog.Warn("MS:", m.ReceiverID, m.Result, m.Error)
	}
}

func TestAll(t *testing.T) {
	surl := fmt.Sprint("http://localhost:", port)
	webClient = NewClient(surl, "client")
	webExecutor := NewExecutor()
	NewReverseExecutor(webClient, "", webExecutor)
	webExecutor.Register(WebCall{})

	router := mux.NewRouter()
	cloudExecutor := NewServer(router, nil)
	cloudExecutor.Register(CloudCall{})
	revClienter = NewReverseClientsOwner(cloudExecutor)
	cloudRC = NewReverseClient(revClienter, "client", "", true)
	cloudRC.TimeoutSecs = 2
	address := fmt.Sprint(":", port)
	znet.ServeHTTPInBackground(address, "", router)

	// testCall(t)
	// testReverseCall(t)
	// testReverseAdd(t)
	testMultiWeb(t)
}
