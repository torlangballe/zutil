//go:build server

package zrest

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/ztesting"
	"github.com/torlangballe/zutil/ztime"
)

const (
	port   = 8457
	method = "test-call-zzzz"
)

func handler(w http.ResponseWriter, req *http.Request) {
	if req != nil && req.Body != nil {
		io.Copy(io.Discard, req.Body)
		req.Body.Close()
	}
}

type Object struct {
	Message string
}

func call(t *testing.T) {
	//	var r Object
	params := zhttp.MakeParameters()
	s := Object{Message: "Hello"}
	surl := fmt.Sprintf("http://127.0.0.1:%d/%s", port, method)
	_, err := zhttp.Post(surl, params, s, nil)
	if err != nil {
		t.Error(err)
		return
	}
}
func TestCalls(t *testing.T) {
	router := mux.NewRouter()
	const count = 100
	AddHandler(router, method, handler).Methods("POST")
	znet.ServeHTTPInBackground(fmt.Sprintf(":%d", port), "", router)

	start := time.Now()
	for i := 0; i < count; i++ {
		call(t)
	}
	since := ztime.Since(start)
	zlog.Warn("100 calls:", since)
	ztesting.LessThan(t, "100 calls less than 600ms", since, 0.6)
}
