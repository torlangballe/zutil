package zrpc

import (
	"encoding/json"
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
	debugReverseCalls bool
)

func (r *ReverseExecutor) Remove() {
	r.stop = true // should use a channel
}

func (r *ReverseExecutor) SetOn(on bool) {
	// zlog.Info("RE: SetOn", zlog.Pointer(r), on)
	r.on = on
}

// Starts the polling process in the background
func NewReverseExecutor(pollClient *Client, id string) *ReverseExecutor {
	// zlog.Info("NewReverseExecutor", pollClient.AuthToken)
	var r ReverseExecutor
	r.client = pollClient.Copy()
	r.client.TimeoutSecs = PollRestartSecs // polling should be fast, total time with execution not included
	r.client.id = id
	r.client.KeepTokenOnAuthenticationInvalid = true
	go startCallingPollForReverseCalls(&r)
	return &r
}

func startCallingPollForReverseCalls(r *ReverseExecutor) {
	// zlog.Info("startCallingPollForReverseCalls", r.client.id)
	for {
		if r.stop {
			return
		}
		if !r.on {
			// zlog.Info("startCallingPollForReverseCalls off", zlog.Pointer(r), r.client.id)
			time.Sleep(time.Millisecond * 50)
			continue
		}
		// we loop forever calling RPCCalls.ReversePoll. No need to sleep between, as it waits on other end for a call to be ready
		var cp callPayloadReceive
		err := r.client.Call("RPCCalls.ReversePoll", r.client.id, &cp)
		if err != nil {
			if debugReverseCalls {
				zlog.Info("Call RPCCalls.ReversePoll, error! cp.Token:", err, cp.Token)
			}
			time.Sleep(time.Millisecond * 50) // lets not go nuts
			continue
		}
		if cp.Method == "" { // we got a dummy callPayloadReceive, because we sent a receivePayload, but did't have anything
			if debugReverseCalls {
				zlog.Info("Call RPCCalls.ReversePoll, reveived dummy:")
			}
			continue
		}
		// zlog.Info("Call RPCCalls.ReversePoll, ok cp.Token:", err, cp.Token)
		// zlog.Info("execute method:", cp.Method, cp.Token)
		go func() {
			var ci ClientInfo
			var rr ReverseResult
			ci.Type = "zrpc-rev"
			ci.ClientID = r.client.id
			ci.Token = r.client.AuthToken

			receive, err := callWithDeadline(ci, cp.Method, cp.Expires, cp.Args)
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
			rr.ReverseReceiverID = r.client.id
			// zlog.Info("execute method:", err, cp.Method, rr.Error)
			cerr := r.client.Call("RPCCalls.ReversePushResult", rr, nil)
			if cerr != nil {
				zlog.Error(cerr, "call push result", rr.Token)
			}
			// zlog.Info("DONE: execute method:", cp.Method, rr.Token)
		}()
	}
}
