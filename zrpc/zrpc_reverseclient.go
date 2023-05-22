package zrpc

import (
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
)

// Reverse calling functionality allows the "server" to set up a "reverse client", and call methods on clients (called reverse receivers) usually calling in to it.
// This allows instant calling of methods on a browser for example.
// It is done by the clients polling if the server has any calls, executing them, and returning the resukt in the next poll.

// pendingCall is a payload waiting to be gotten by the asking client
type pendingCall struct {
	CallPayload
	done chan *receivePayload // the done channel has a reieveiPayLoad written to it when received.
}

var (
	allReverseClients     zmap.LockMap[string, *ReverseClient]
	HandleNewReceiverFunc = defaultHandleNewReceiverFunc // HandleNewReceiverFunc is called when a new receiver polls for calls
)

// The ReverseClient is very simple. All actual communication is done by zrpc from client to server.
// It is mapped with a ReverseReceiverID, and is used to call to a specific client.
type ReverseClient struct {
	TimeoutSecs        float64                           // TimeoutSecs is time from a reverse call is initiated, until it times out if no result/error returned yet
	pendingCallsToSend zmap.LockMap[string, pendingCall] // pendingCallsToSend is map of pending calls waiting to be fetched by a poll, keyed by token used to identify the call
	pendingCallsSent   zmap.LockMap[string, pendingCall] // pendingCallsSent have been fetched, but are awaiting results
}

// ReversePoll is called by clients, asking for calls. It first finds the correct ReverseClient using rp.ReverseReceiverID.
// If the caller has an existing result (rp.Token set), the pendingCall is found in pendingCallsSent using rp.Token, and
// the call is finished by writing the result to the pendingCall's done channel.
// If there are calls waiting in pendingCallsToSend, *cp is set and the method returns. Otherwise it waits,
// intermittently sleeping for a short while, awaiting a call added.
// It returns without error if no calls waiting, cp.Method will be empty.
func (RPCCalls) ReversePoll(rp ReverseResult, cp *CallPayload) error {
	rc, _ := allReverseClients.Get(rp.ReverseReceiverID)
	if rc == nil {
		rc = HandleNewReceiverFunc(rp.ReverseReceiverID, false)
	}
	var exit bool
	start := time.Now()
	for time.Since(start) < time.Second*(PollRestartSecs-1) {
		if rp.Token != "" {
			pendingCall, got := rc.pendingCallsSent.Get(rp.Token)
			if !got {
				return zlog.NewError("No call for result with token:", rp.Token) // make some kind of transport error
			} else {
				pendingCall.done <- &rp.receivePayload
				exit = true
			}
		}
		var has bool
		if rc.pendingCallsToSend.Count() != 0 {
			has = true
			rc.pendingCallsToSend.ForEach(func(token string, call pendingCall) bool {
				*cp = call.CallPayload
				rc.pendingCallsToSend.Remove(token)
				rc.pendingCallsSent.Set(token, call)
				return false
			})
		}
		if has || exit {
			return nil
		}
		time.Sleep(time.Millisecond * 10)
	}
	return nil
}

// Call has the same syntax as a Client.Call.
// It creates a CallPayload, with a unique token, puts it on the ReverseClient's
// pendingCallsToSend map. It then waits for a timeout or for ReversePoll above to write
// a result to a channel.
func (rc *ReverseClient) Call(method string, args, resultPtr any) error {
	var pc pendingCall
	pc.CallPayload = CallPayload{Method: method, Args: args}
	token := zstr.GenerateRandomHexBytes(20)
	pc.CallPayload.Token = token
	pc.done = make(chan *receivePayload, 10)
	rc.pendingCallsToSend.Set(token, pc)
	ticker := time.NewTicker(ztime.SecondsDur(rc.TimeoutSecs))
	select {
	case <-ticker.C:
		return zlog.NewError("zrpc.Call reverse timed out:", method)
	case r := <-pc.done:
		reflect.ValueOf(resultPtr).Elem().Set(reflect.ValueOf(r.Result))
		return nil
	}
}

// defaultHandleNewReceiverFunc adds or removes a ReverseClient to allReverseClients.
// When overriding HandleNewReceiverFunc, you probably call this anyway, but use the ReverseClient.
func defaultHandleNewReceiverFunc(id string, deleted bool) *ReverseClient {
	if deleted {
		c, _ := allReverseClients.Get(id)
		allReverseClients.Remove(id)
		return c
	}
	c := &ReverseClient{}
	c.TimeoutSecs = 100
	allReverseClients.Set(id, c)
	return c
}
