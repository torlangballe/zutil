package zrpc

import (
	"context"
	"time"

	"github.com/torlangballe/zutil/zmap"
)

// Reverse execution funcionallity is typically run on a browser or something not callable from server.
// After adding methods using Register(), InitReverseExecutor is called to start polling the server
// for calls it wants to perform for this given ReverseReceiverID.
const PollRestartSecs = 10

type ReverseResult struct {
	receivePayload
	ReverseReceiverID string
	Token             string
}

var reverseResults zmap.LockMap[string, ReverseResult]

func InitReverseExecutor(pollClient *Client) {
	timedClient := *pollClient
	timedClient.TimeoutSecs = PollRestartSecs
	go startCallingPollForReverseCalls(&timedClient)
}

func startCallingPollForReverseCalls(client *Client) {
	for {
		var cp callPayloadReceive
		var rp ReverseResult
		var token string
		if reverseResults.Count() != 0 {
			token := reverseResults.AnyKey()
			rp, _ = reverseResults.Get(token)
			reverseResults.Remove(token)
			rp.Token = token
		}
		rp.ReverseReceiverID = client.id
		err := client.Call("RPCCalls.ReversePoll", rp, &cp)
		if err != nil {
			if rp.Token != "" { // if we had popped a result, and get a transport error, put it back
				reverseResults.Set(token, rp)
			}
			time.Sleep(time.Millisecond * 50) // lets not go nuts
			continue
		}
		if cp.Method == "" { // we got a dummy callPayloadReceive, because it sent a receivePayload, but did't have anything
			continue
		}
		ctx := context.Background()
		var ci ClientInfo
		ci.Type = "zrpc-rev"
		ci.ClientID = cp.ClientID
		ci.Token = cp.Token
		receive, err := callMethodName(ctx, ci, cp.Method, cp.Args)
		if err != nil {
			rp.Error = err.Error()
		}
		rp.receivePayload = receive
		reverseResults.Set(ci.Token, rp)
	}
}
