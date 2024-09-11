package zrpc

import (
	"encoding/json"
	"math"
	"math/rand"
	"time"

	"github.com/torlangballe/zutil/zcache"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/ztimer"
)

// Reverse execution funcionality is typically run on a browser or something not callable from server.
// After adding methods using Register(), InitReverseExecutor is called to start polling the server
// for calls it wants to perform for this given ReverseReceiverID.
// It executes the methods, and stores the results in reverseResults.
// On next poll, any exisitng result is popped and sent as the input to the poll.

const PollRestartSecs = 10 // PollRestartSecs is how long to wait asking for a call, before doing a new poll.

type ReverseExecutor struct {
	Executor  *Executor
	rid       string
	client    *Client
	pollTimer *ztimer.Repeater
	stop      bool
	on        bool
}

// ReverseResult is always sent with a poll. It can contain an existing result (if Token set), or in any case
// a ReverseReceiverID to identify who is asking for calls.
type ReverseResult struct {
	clientReceivePayload
	ReverseReceiverID string
	Token             string
}

var (
	temporaryDataServe = zcache.NewExpiringMap[int64, []byte](4) // temporaryDataServe stores temporary bytes to serve within 2 seconds. See AddToTemporaryServe.
	reverseResults     []ReverseResult                           // reverseResults stores results from executed methods, waiting to be sent on next poll
	EnableLogExecutor  zlog.Enabler                              = false
)

func init() {
	zlog.RegisterEnabler("zrpc.EnableLogReverseExe", &EnableLogExecutor)
}

func (r *ReverseExecutor) Remove() {
	r.stop = true // should use a channel
}

func (r *ReverseExecutor) SetOn(on bool) {
	// zlog.Info("RE: SetOn", zlog.Pointer(r), on)
	r.on = on
}

// Starts the polling process in the background
func NewReverseExecutor(pollClient *Client, id string, executor *Executor) *ReverseExecutor { // , id string
	// zlog.Info("NewReverseExecutor", id, pollClient.AuthToken) // EnableLogExecutor,
	r := &ReverseExecutor{}
	r.on = true
	r.client = pollClient
	r.client.TimeoutSecs = PollRestartSecs // polling should be fast, total time with execution not included
	r.rid = id
	if id == "" {
		r.rid = pollClient.ID
	}
	//!!! r.client.KeepTokenOnAuthenticationInvalid = true
	r.Executor = executor
	go startCallingPollForReverseCalls(r)
	return r
}

func startCallingPollForReverseCalls(r *ReverseExecutor) {
	// EnableLogExecutor = true
	for {
		zlog.Warn(EnableLogExecutor, "startCallingPollForReverseCalls", r.on, r.stop, r.rid)
		if r.stop {
			return
		}
		if !r.on {
			zlog.Warn(EnableLogExecutor, "startCallingPollForReverseCalls off", zlog.Pointer(r), r.rid)
			time.Sleep(time.Millisecond * 50)
			continue
		}
		// if r.client.AuthToken == "" {
		// 	zlog.Warn("startCallingPollForReverseCalls token is empty") // EnableLogExecutor,
		// 	time.Sleep(time.Millisecond * 50)
		// 	continue
		// }
		start := time.Now()
		// we loop forever calling RPCCalls.ReversePoll. No need to sleep between, as it waits on other end for a call to be ready
		var cp callPayloadReceive
		maxTimeout := math.Max(PollRestartSecs, r.client.TimeoutSecs)
		err := r.client.CallWithTimeout(maxTimeout, "ReverseClientsOwner.ReversePoll", r.rid, &cp)
		zlog.Warn(EnableLogExecutor, "Call ReverseClientsOwner.ReversePoll:", cp.Method, maxTimeout, err, time.Since(start))
		if err != nil {
			zlog.Warn(EnableLogExecutor, "Call ReverseClientsOwner.ReversePoll, error! cp.Token:", err, cp.Token)
			time.Sleep(time.Millisecond * 50) // lets not go nuts
			continue
		}
		if cp.Method == "" { // we got a dummy callPayloadReceive, because we sent a receivePayload, but did't have anything
			zlog.Warn(EnableLogExecutor, "Call ReverseClientsOwner.ReversePoll, reveived dummy:")
			continue
		}
		zlog.Warn(EnableLogExecutor, "Call ReverseClientsOwner.ReversePoll, ok cp.Token:", err, cp.Token, cp.Method)
		// zlog.Warn("rev execute method:", cp.Method, cp.Token)
		go func() {
			var ci ClientInfo
			var rr ReverseResult
			ci.Type = "zrpc-rev"
			ci.ClientID = r.client.ID // or r.rid ??
			ci.Token = r.client.AuthToken

			receive, err := r.Executor.callWithDeadline(ci, cp.Method, cp.Expires, cp.Args, r.client)
			zlog.Info(EnableLogExecutor, "zrpc ReverseExecutor: callMethod:", cp.Method, err, time.Since(start))
			if err != nil {
				rr.clientReceivePayload.Error = err.Error()
			}
			if rr.clientReceivePayload.Error == "" {
				rr.clientReceivePayload.Error = receive.Error
			}
			rr.clientReceivePayload.AuthenticationInvalid = receive.AuthenticationInvalid
			rr.clientReceivePayload.Result, err = json.Marshal(receive.Result)
			zlog.OnError(err, "marshal receive")
			rr.clientReceivePayload.TransportError = receive.TransportError
			rr.Token = cp.Token
			rr.ReverseReceiverID = r.rid
			maxTimeout := math.Max(PollRestartSecs, r.client.TimeoutSecs)
			cerr := r.client.CallWithTimeout(maxTimeout, "ReverseClientsOwner.ReversePushResult", rr, nil)
			if cerr != nil {
				zlog.Error(cerr, "call push result", rr.Token)
			}
			// zlog.Info("DONE: execute method:", cp.Method, rr.Token)
		}()
	}
}

// AddToTemporaryServe adds some data to be served within a few seconds, returning a unique id.
// Client.RequestTemporaryServe requests it with that id.
func AddToTemporaryServe(data []byte) int64 {
	id := rand.Int63()
	temporaryDataServe.Set(id, data)
	return id
}
