package zrpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
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

type ReverseClienter struct {
	HandleNewReverseReceiverFunc func(id string, rc *ReverseClient, ci *ClientInfo)
	executor                     *Executor
	allReverseClients            zmap.LockMap[string, *ReverseClient] // id/rc allReverseClients stores all clients who have called ReversePoll with a unique ReverseReceiverID. Removed if no poll for a while.
	multiWaiting                 zmap.LockMap[string, bool]
}

// The ReverseClient is actually on a device running a zrpc.Executor.
// It gets polled be zrpc.ReverseExecutors for jobs to run with ReversePoll and then ReversePushResult with it's result.

// It has a ReverseReceiverID, and is used to call to a specific (reverse) client.
type ReverseClient struct {
	ClientUserToken    string
	TimeoutSecs        float64 // TimeoutSecs is time from a reverse call is initiated, until it times out if no result/error returned yet
	LastPolled         time.Time
	pendingCallsToSend zmap.LockMap[string, pendingCall] // pendingCallsToSend is map of pending calls waiting to be fetched by a poll, keyed by token used to identify the call
	pendingCallsSent   zmap.LockMap[string, pendingCall] // pendingCallsSent have been fetched, but are awaiting results
	permanent          bool
}

// pendingCall is a payload waiting to be gotten by the asking client
type pendingCall struct {
	placed time.Time
	CallPayload
	done chan *clientReceivePayload // the done channel has a reieveiPayLoad written to it when received.
}

var (
	NoCallForTokenErr      = errors.New("NoCallForTokenErr")
	EnableLogReverseClient zlog.Enabler
	MainReverseClienter    *ReverseClienter
)

func NewReverseClienter(executor *Executor) *ReverseClienter {
	r := &ReverseClienter{}
	r.executor = executor
	executor.Register(r)
	zlog.RegisterEnabler("zrpc.EnableLogReverseClient", &EnableLogReverseClient)
	EnableLogReverseClient = false
	ztimer.RepeatForever(PollRestartSecs+5, func() {
		r.allReverseClients.ForAll(func(cid string, c *ReverseClient) {
			if time.Since(c.LastPolled) > ztime.SecondsDur(PollRestartSecs+5) {
				var str string
				zlog.Info("zrpc: Unresponsive client1 from allReverseClients", c.permanent, cid, r.allReverseClients.Count())
				if !c.permanent {
					str = "removed"
					RemoveReverseClient(r, cid)
				}
				zlog.Info("zrpc: Unresponsive client from allReverseClients", c.permanent, cid, c.LastPolled, str, r.allReverseClients.Count())
			}
		})
	})
	return r
}

var revCount int

// NewReverseClient adds a ReverseClient to allReverseClients.
// When overriding HandleNewReverseReceiverFunc, you probably call this anyway, but use the ReverseClient.
func NewReverseClient(r *ReverseClienter, receiverID string, userAuthToken string, permanent bool) *ReverseClient {
	c := &ReverseClient{}
	c.TimeoutSecs = 100
	c.LastPolled = time.Now()
	c.ClientUserToken = userAuthToken
	c.permanent = permanent
	zlog.Info("NewReverseClient", receiverID)
	r.allReverseClients.Set(receiverID, c)
	return c
}

// ReversePoll is called by clients, asking for calls. It first finds the correct ReverseClient using receiverID
// If the caller has an existing result (rp.Token set), the pendingCall is found in pendingCallsSent using rp.Token, and
// the call is finished by writing the result to the pendingCall's done channel.
// If there are calls waiting in pendingCallsToSend, *cp is set and the method returns. Otherwise it waits,
// intermittently sleeping for a short while, awaiting a call added.
// It returns without error if no calls waiting, cp.Method will be empty.
func (r *ReverseClienter) ReversePoll(ci *ClientInfo, receiverID string, cp *CallPayload) error {
	revCount++
	// zlog.Warn("ReversePoll1:", revCount, receiverID, cp.Token)
	rc := r.findOrAddReverseClient(receiverID, ci)
	zlog.Info(EnableLogReverseClient, "ReversePoll", receiverID, r.allReverseClients.Count(), rc.pendingCallsToSend.Count(), rc.pendingCallsSent.Count())
	start := time.Now()
	// var method string
	for time.Since(start) < time.Second*(PollRestartSecs-1) {
		var has bool
		// if findReverseClient(receiverID) != rc {
		// 	zlog.Info("ReversePoll2", findReverseClient(receiverID) != rc)
		// }
		rc.pendingCallsToSend.ForEach(func(token string, call pendingCall) bool {
			if time.Since(call.Expires) >= 0 {
				var rp clientReceivePayload
				rp.TransportError = TransportError(zstr.Spaced("zrpc.ReverseCall timed out before being polled from executor", call.Method, call.Expires))
				rc.pendingCallsToSend.Remove(token)
				call.done <- &rp
				return true
			} else {
				has = true
				if time.Since(call.placed) > time.Millisecond*100 {
					zlog.Info("PendingCallPolled late:", call.Method, "placed:", time.Since(call.placed))
				}
				zlog.Info(EnableLogReverseClient, "zrpc.ReversePoll added call:", call.Method)
				*cp = call.CallPayload
				// method = call.CallPayload.Method
				// zlog.Warn("ReversePoll.Call:", method)
				rc.pendingCallsSent.Set(token, call)
			}
			rc.pendingCallsToSend.Remove(token)
			return false
		})
		if has {
			// zlog.Warn("ReversePoll has:", revCount, method, receiverID, cp.Token, rc.pendingCallsToSend.Count())
			return nil
		}
		// zlog.Warn("ReversePoll hasnt1:", revCount)
		time.Sleep(time.Millisecond * 2) // use a channel instead
	}
	// zlog.Info("ReversePoll hasnt:", revCount, receiverID, cp.Token)
	return nil
}

func (r *ReverseClienter) ReversePushResult(rp ReverseResult) error {
	// zlog.Info("ReversePushResult:", rp.Token, rp.Error)
	rc := r.findOrAddReverseClient(rp.ReverseReceiverID, nil)
	pendingCall, got := rc.pendingCallsSent.Pop(rp.Token)
	zlog.Info(EnableLogReverseClient, "zrpc.PushResult:", pendingCall.Method, got)
	if !got {
		zlog.Error(nil, "No call for result with token:", rp.Token, "in", rp.ReverseReceiverID, rc.pendingCallsSent.Count()) // make some kind of transport error
		return NoCallForTokenErr
	}
	pendingCall.done <- &rp.clientReceivePayload
	return nil
}

func (r *ReverseClienter) findOrAddReverseClient(id string, ci *ClientInfo) *ReverseClient {
	// zlog.Info("RC findOrAddReverseClient:", id)
	rc, _ := r.allReverseClients.Get(id)
	if rc == nil {
		if ci == nil { // if ci is nil, it's from ReversePoll, don't add otherwise
			zlog.Error(nil, "findOrAddReverseClient ci=nil: no reverse client for id:", id)
			return nil
		}
		rc = NewReverseClient(r, id, ci.Token, false)
		if r.HandleNewReverseReceiverFunc != nil {
			r.HandleNewReverseReceiverFunc(id, rc, ci)
		}
	}
	zlog.Assert(rc != nil, id)
	rc.LastPolled = time.Now()
	return rc
}

// Call has the same syntax as a regular zrpc Call.
// It creates a CallPayload, with a unique token, puts it on the ReverseClient's
// pendingCallsToSend map. It then waits for a timeout or for ReversePoll above to write
// a result to a channel.
func (rc *ReverseClient) Call(method string, args, resultPtr any) error {
	return rc.CallWithTimeout(rc.TimeoutSecs, method, args, resultPtr)
}

func (rc *ReverseClient) CallWithTimeout(timeoutSecs float64, method string, args, resultPtr any) error {
	zlog.Info(EnableLogReverseClient, "zrpc.RevCall:", method, timeoutSecs)
	var pc pendingCall
	pc.CallPayload = CallPayload{Method: method, Args: args}
	pc.placed = time.Now()
	pc.Expires = time.Now().Add(ztime.SecondsDur(timeoutSecs))
	token := zstr.GenerateRandomHexBytes(16)
	pc.CallPayload.Token = token
	pc.done = make(chan *clientReceivePayload, 10)
	rc.pendingCallsToSend.Set(token, pc)
	// zlog.Info("CALL:", method, pc.ClientID, token, rc.pendingCallsToSend.Count())
	dur := ztime.SecondsDur(math.Min(timeoutSecs, PollRestartSecs))
	ticker := time.NewTicker(dur)
	select {
	case <-ticker.C:
		ticker.Stop() // Very important to stop ticker, or memory leak
		rc.pendingCallsToSend.Remove(token)
		return zlog.NewError("Reverse zrpc.Call timed out:", method, dur)
	case r := <-pc.done:
		ticker.Stop() // Very important to stop ticker, or memory leak
		if r.Error != "" {
			return errors.New(r.Error)
		}
		if resultPtr != nil {
			err := json.Unmarshal(r.Result, resultPtr)
			// zlog.Info("RevCall done", err)
			if err != nil {
				return err
			}
		}
		// zlog.Info("ChannelPushed, call done", reflect.TypeOf(r.Result), resultPtr)
		return nil
	}
}

func RemoveReverseClient(r *ReverseClienter, receiverID string) {
	r.allReverseClients.Remove(receiverID)
}

type MultiCallResult[R any] struct {
	Result     any
	ReceiverID string
	Error      error
}

func CallAll[R any](r *ReverseClienter, timeoutSecs float64, method, receiveIDWildcard string, args any) []MultiCallResult[R] {
	if r == nil {
		r = MainReverseClienter
	}
	var out []MultiCallResult[R]
	var wg sync.WaitGroup
	r.allReverseClients.ForEach(func(id string, rc *ReverseClient) bool {
		zlog.Info("CALL-ALL:", id)
		if receiveIDWildcard == "*" || zstr.MatchWildcard(receiveIDWildcard, id) {
			sid := id + ":" + method
			if r.multiWaiting.Has(sid) {
				return true
			}
			r.multiWaiting.Set(sid, true)
			wg.Add(1)
			go func() {
				var result R
				ts := timeoutSecs
				if ts == 0 {
					ts = rc.TimeoutSecs
				}
				err := rc.CallWithTimeout(ts, method, args, &result)
				m := MultiCallResult[R]{result, id, err}
				out = append(out, m)
				r.multiWaiting.Remove(sid)
				wg.Done()
			}()
		}
		return true
	})
	wg.Wait()
	return out
}

func CallAllSimple(timeoutSecs float64, method, receiveIDWildcard string, args any) []error {
	var errs []error
	results := CallAll[Unused](nil, timeoutSecs, method, receiveIDWildcard, args)
	for _, r := range results {
		e := fmt.Errorf("%s: %w", r.ReceiverID, r.Error)
		errs = append(errs, e)
	}
	return errs
}
