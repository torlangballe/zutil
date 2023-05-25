package zrpc

import (
	"errors"
	"reflect"
	"time"

	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

// Reverse calling functionality allows the "server" to set up a "reverse client", and call methods on clients (called reverse receivers) that usually only call in to it.
// This allows instant calling of methods on a browser for example.
// It is done by the clients polling if the server has any calls, executing them, and returning the resukt in the next poll.

// pendingCall is a payload waiting to be gotten by the asking client
type pendingCall struct {
	CallPayload
	done chan *receivePayload // the done channel has a reieveiPayLoad written to it when received.
}

var (
	AllReverseClients     zmap.LockMap[string, *ReverseClient]         // AllReverseClients stores all clients who have called ReversePoll with a unique ReverseReceiverID. Removed if no poll for a while.
	HandleNewReceiverFunc func(id string, deleted bool) *ReverseClient // HandleNewReceiverFunc is called when a new receiver polls for calls
	NoCallForTokenErr     = errors.New("NoCallForTokenErr")
)

func init() {
	HandleNewReceiverFunc = DefaultHandleNewReceiverFunc
}

// The ReverseClient is very simple. All actual communication is done by zrpc from client to server.
// It has a ReverseReceiverID, and is used to call to a specific (reverse) client.
type ReverseClient struct {
	TimeoutSecs        float64 // TimeoutSecs is time from a reverse call is initiated, until it times out if no result/error returned yet
	LastPolled         time.Time
	pendingCallsToSend zmap.LockMap[string, pendingCall] // pendingCallsToSend is map of pending calls waiting to be fetched by a poll, keyed by token used to identify the call
	pendingCallsSent   zmap.LockMap[string, pendingCall] // pendingCallsSent have been fetched, but are awaiting results
}

// ReversePoll is called by clients, asking for calls. It first finds the correct ReverseClient using rp.ReverseReceiverID.
// If the caller has an existing result (rp.Token set), the pendingCall is found in pendingCallsSent using rp.Token, and
// the call is finished by writing the result to the pendingCall's done channel.
// If there are calls waiting in pendingCallsToSend, *cp is set and the method returns. Otherwise it waits,
// intermittently sleeping for a short while, awaiting a call added.
// It returns without error if no calls waiting, cp.Method will be empty.
func (RPCCalls) ReversePoll(receiverID string, cp *CallPayload) error {
	rc := findOrHandleNewReverseReceiver(receiverID)
	start := time.Now()
	for time.Since(start) < time.Second*(PollRestartSecs-1) {
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
		if has {
			return nil
		}
		time.Sleep(time.Millisecond * 10)
	}
	return nil
}

func (RPCCalls) ReversePushResult(rp ReverseResult) error {
	rc := findOrHandleNewReverseReceiver(rp.ReverseReceiverID)
	pendingCall, got := rc.pendingCallsSent.Pop(rp.Token)
	if !got {
		zlog.Error(nil, "No call for result with token:", rp.Token, "in", rp.ReverseReceiverID, rc.pendingCallsSent.Count()) // make some kind of transport error
		return NoCallForTokenErr
	}
	pendingCall.done <- &rp.receivePayload
	return nil
}

func findOrHandleNewReverseReceiver(id string) *ReverseClient {
	rc, _ := AllReverseClients.Get(id)
	if rc == nil {
		rc = HandleNewReceiverFunc(id, false)
		zlog.Info("ADDING new recevier:", id)
	}
	rc.LastPolled = time.Now()
	return rc
}

// Call has the same syntax as a regular zrpc Call.
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
	ticker := time.NewTicker(ztime.SecondsDur(PollRestartSecs))
	select {
	case <-ticker.C:
		zlog.Info("Call timed out")
		rc.pendingCallsToSend.Remove(token)
		return zlog.NewError("zrpc.Call reverse timed out:", method)
	case r := <-pc.done:
		reflect.ValueOf(resultPtr).Elem().Set(reflect.ValueOf(r.Result))
		return nil
	}
}

// DefaultHandleNewReceiverFunc adds or removes a ReverseClient to AllReverseClients.
// When overriding HandleNewReceiverFunc, you probably call this anyway, but use the ReverseClient.
func DefaultHandleNewReceiverFunc(id string, deleted bool) *ReverseClient {
	if deleted {
		c, _ := AllReverseClients.Get(id)
		AllReverseClients.Remove(id)
		return c
	}
	c := &ReverseClient{}
	c.TimeoutSecs = 100
	c.LastPolled = time.Now()
	AllReverseClients.Set(id, c)
	ztimer.RepeatIn(PollRestartSecs+5, func() bool {
		if time.Since(c.LastPolled) > ztime.SecondsDur(PollRestartSecs+5) {
			zlog.Info("zrpc: Removed unpolling client from AllReverseClients", id)
			AllReverseClients.Remove(id)
			HandleNewReceiverFunc(id, true)
			return false
		}
		return true
	})
	return c
}

var multiWaiting zmap.LockMap[string, bool]

func CallAllReceivers[R any](method, receiveIDWildcard string, args any, got func(result *R, err error)) {
	AllReverseClients.ForEach(func(id string, rc *ReverseClient) bool {
		if receiveIDWildcard == "*" || zstr.MatchWildcard(receiveIDWildcard, id) {
			sid := id + ":" + method
			if multiWaiting.Has(sid) {
				return true
			}
			multiWaiting.Set(sid, true)
			go func() {
				var r R
				err := rc.Call(method, args, &r)
				multiWaiting.Remove(sid)
				if got != nil {
					got(&r, err)
				}
			}()
		}
		return true
	})
}
