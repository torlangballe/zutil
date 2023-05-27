package zrpc

import (
	"context"
	"encoding/json"
	"time"

	"github.com/torlangballe/zutil/zlog"
)

// Reverse execution funcionality is typically run on a browser or something not callable from server.
// After adding methods using Register(), InitReverseExecutor is called to start polling the server
// for calls it wants to perform for this given ReverseReceiverID.
// It executes the methods, and stores the results in reverseResults.
// On next poll, any exisitng result is popped and sent as the input to the poll.

const PollRestartSecs = 10 // PollRestartSecs is how long to wait asking for a call, before doing a new poll.

// ReverseResult is always sent with a poll. It can contain an existing result (if Token set), or in any case
// a ReverseReceiverID to identify who is asking for calls.
type ReverseResult struct {
	clientReceivePayload
	ReverseReceiverID string
	Token             string
}

// reverseResults stores results from executed methods, waiting to be sent on next poll
var reverseResults []ReverseResult

// Starts the polling process in the background
func InitReverseExecutor(pollClient *Client, id string) {
	timedClient := *pollClient
	timedClient.TimeoutSecs = PollRestartSecs // polling should be fast, total time with execution not included
	timedClient.id = id
	timedClient.KeepTokenOnAuthenticationInvalid = true
	go startCallingPollForReverseCalls(&timedClient)
}

func startCallingPollForReverseCalls(client *Client) {
	for {
		// we loop forever calling RPCCalls.ReversePoll. No need to sleep between, as it waits on other end for a call to be ready
		var cp callPayloadReceive
		err := client.Call("RPCCalls.ReversePoll", client.id, &cp)
		if err != nil {
			zlog.Info("Call RPCCalls.ReversePoll, error! rp.Token:", err, cp.Token)
			time.Sleep(time.Millisecond * 50) // lets not go nuts
			continue
		}
		// if cp.Method == "" { // we got a dummy callPayloadReceive, because we sent a receivePayload, but did't have anything
		// 	continue
		// }
		// zlog.Info("execute method:", cp.Method, cp.Token)
		go func() {
			var ci ClientInfo
			var rr ReverseResult
			ctx := context.Background()
			ci.Type = "zrpc-rev"
			ci.ClientID = client.id
			ci.Token = client.AuthToken
			receive, err := callMethodName(ctx, ci, cp.Method, cp.Args)
			if err != nil {
				rr.Error = err.Error()
			}
			if err != nil {
				rr.Error = err.Error()
			}
			// zlog.Info("call receive:", cp.Method, reflect.TypeOf(receive.Result), receive.Result)
			rr.clientReceivePayload.Error = receive.Error
			rr.clientReceivePayload.AuthenticationInvalid = receive.AuthenticationInvalid
			rr.clientReceivePayload.Result, err = json.Marshal(receive.Result)
			zlog.OnError(err, "marshal receive")
			rr.clientReceivePayload.TransportError = receive.TransportError
			rr.Token = cp.Token
			rr.ReverseReceiverID = client.id
			cerr := client.Call("RPCCalls.ReversePushResult", rr, nil)
			if cerr != nil {
				zlog.Error(cerr, "call push result", rr.Token)
			}
			// zlog.Info("DONE: execute method:", cp.Method, rr.Token)
		}()
	}
}
