package xrpc

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/znet"
	"github.com/torlangballe/zutil/zrest"
)

var (
	callClientChannels  = map[string]chan *callPayload{}   // map of callPayload.ToID to channel to grab a callPayload off on poll from client
	clientReplyChannels = map[int64]chan *receivePayload{} // map of InstanceID to channel to send receivePayload back with
	instanceCounter     int64
)

func InitServer(certFilesSuffix string, port int) {
	if port == 0 {
		port = 1300
	}
	http.HandleFunc("/xrpc", handleIncomingCall)
	http.HandleFunc("/xrpcPoll", handlePollToReceiveCall)
	http.HandleFunc("/xrpcReceive", handleReceivedResult)
	zlog.Info("xrpc.InitServer", certFilesSuffix, port)
	znet.ServeHTTPInBackground(port, certFilesSuffix, nil)
}

func makeClientCallChannel(id string) chan *callPayload {
	ch := callClientChannels[id]
	if ch == nil { // todo: use mutex
		ch = make(chan *callPayload, 20)
		callClientChannels[id] = ch
	}
	return ch
}

func handlePollToReceiveCall(w http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("id")
	ch := makeClientCallChannel(id)
	t := time.NewTicker(time.Second * pollSecs)
	for {
		select {
		case <-req.Context().Done():
			zlog.Debug("Done")
			
		case cp := <-ch:
			e := json.NewEncoder(w)
			err := e.Encode(cp)
			if err != nil {
				zlog.Error(err, "encode")
			}
			return
		case <-t.C:
			zrest.ReturnAndPrintError(w, req, http.StatusTeapot, "poll timed out, so do another")
		}
	}
}

func handleReceivedResult(w http.ResponseWriter, req *http.Request) {
	var rp receivePayload
	d := json.NewDecoder(req.Body)
	err := d.Decode(&rp)
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusBadRequest, "json-decode", err)
		return
	}
	clientReplyChannel := clientReplyChannels[rp.InstanceID]
	if clientReplyChannel == nil {
		zrest.ReturnAndPrintError(w, req, http.StatusNotFound, "Got result for method without clientReply:", rp.InstanceID)
		return
	}
	clientReplyChannel <- &rp
}

func handleIncomingCall(w http.ResponseWriter, req *http.Request) {
	// ctx := context.Background()
	// zlog.Info("start incoming")
	var cp callPayload
	d := json.NewDecoder(req.Body)
	err := d.Decode(&cp)
	if err != nil {
		zrest.ReturnAndPrintError(w, req, http.StatusBadRequest, "json-decode", err)
		return
	}
	if cp.ToID == "" {
		callErr := handleCallWithMethodWriteBack(w, &cp)
		if callErr != nil {
			zrest.ReturnAndPrintError(w, req, http.StatusBadRequest, "handle call direct", callErr)
			return
		}
		return
	}
	// put in channel to be picked up by polling client:
	instanceCounter++
	cp.InstanceID = instanceCounter
	callChannel := makeClientCallChannel(cp.ToID)
	callChannel <- &cp

	replyChannel := make(chan *receivePayload, 1)
	clientReplyChannels[cp.InstanceID] = replyChannel

	t := time.NewTicker(time.Second*runMethodSecs + 5)
	for {
		select {
		case rp := <-replyChannel:
			e := json.NewEncoder(w)
			err := e.Encode(rp)
			if err != nil {
				zlog.Error(err, "encode")
			}
			return
		case <-t.C:
			zrest.ReturnAndPrintError(w, req, http.StatusRequestTimeout, "call method on client timed out", cp.Method)
		}
	}
}
