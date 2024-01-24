package zrpc

import (
	"encoding/json"
	"math"
	"time"

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

// reverseResults stores results from executed methods, waiting to be sent on next poll
var (
	reverseResults    []ReverseResult
	EnableLogExecutor zlog.Enabler = false
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
func NewReverseExecutor(pollClient *Client, id string, executor *Executor) *ReverseExecutor {
	zlog.Info("NewReverseExecutor", id, pollClient.AuthToken) // EnableLogExecutor,
	r := &ReverseExecutor{}
	r.on = true
	r.client = pollClient
	r.client.TimeoutSecs = PollRestartSecs // polling should be fast, total time with execution not included
	if id == "" {
		id = pollClient.ID
	}
	r.client.ID = id
	r.client.KeepTokenOnAuthenticationInvalid = true
	r.Executor = executor
	go startCallingPollForReverseCalls(r)
	return r
}

func startCallingPollForReverseCalls(r *ReverseExecutor) {
	// EnableLogExecutor = true
	for {
		zlog.Info(EnableLogExecutor, "startCallingPollForReverseCalls", r.on, r.stop, r.client.ID)
		if r.stop {
			return
		}
		if !r.on {
			zlog.Info(EnableLogExecutor, "startCallingPollForReverseCalls off", zlog.Pointer(r), r.client.ID)
			time.Sleep(time.Millisecond * 50)
			continue
		}
		start := time.Now()
		// we loop forever calling RPCCalls.ReversePoll. No need to sleep between, as it waits on other end for a call to be ready
		var cp callPayloadReceive
		maxTimeout := math.Max(PollRestartSecs, r.client.TimeoutSecs)
		err := r.client.CallWithTimeout(maxTimeout, "ReverseClienter.ReversePoll", r.client.ID, &cp)
		zlog.Info(EnableLogExecutor, "Call ReverseClienter.ReversePoll:", maxTimeout, err, time.Since(start)) // EnableLogExecutor,
		if err != nil {
			zlog.Info(EnableLogExecutor, "Call ReverseClienter.ReversePoll, error! cp.Token:", err, cp.Token)
			time.Sleep(time.Millisecond * 50) // lets not go nuts
			continue
		}
		if cp.Method == "" { // we got a dummy callPayloadReceive, because we sent a receivePayload, but did't have anything
			zlog.Info(EnableLogExecutor, "Call ReverseClienter.ReversePoll, reveived dummy:")
			continue
		}
		zlog.Info(EnableLogExecutor, "Call ReverseClienter.ReversePoll, ok cp.Token:", err, cp.Token, cp.Method)
		// zlog.Warn("rev execute method:", cp.Method, cp.Token)
		go func() {
			var ci ClientInfo
			var rr ReverseResult
			ci.Type = "zrpc-rev"
			ci.ClientID = r.client.ID
			ci.Token = r.client.AuthToken

			receive, err := r.Executor.callWithDeadline(ci, cp.Method, cp.Expires, cp.Args)
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
			rr.ReverseReceiverID = r.client.ID
			maxTimeout := math.Max(PollRestartSecs, r.client.TimeoutSecs)
			cerr := r.client.CallWithTimeout(maxTimeout, "ReverseClienter.ReversePushResult", rr, nil)
			// zlog.Info("zrpc.RevExe RevPushResult:", cp.Method, rr.Error, maxTimeout, cerr, time.Since(start))
			if cerr != nil {
				zlog.Error(cerr, "call push result", rr.Token)
			}
			// zlog.Info("DONE: execute method:", cp.Method, rr.Token)
		}()
	}
}
