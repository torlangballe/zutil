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
	ClientUserToken string
	TimeoutSecs     float64 // TimeoutSecs is time from a reverse call is initiated, until it times out if no result/error returned yet
	LastPolled      time.Time
	pendingCalls    chan pendingCall
	// pendingCallsToSend zmap.LockMap[string, pendingCall] // pendingCallsToSend is map of pending calls waiting to be fetched by a poll, keyed by token used to identify the call
	pendingCallsSent zmap.LockMap[string, pendingCall] // pendingCallsSent have been fetched, but are awaiting results
	permanent        bool
	rid              string
}

type RowGetter func(receiverID string, index int) any

// pendingCall is a payload waiting to be gotten by the asking client
type pendingCall struct {
	placed time.Time
	CallPayload
	done chan *clientReceivePayload // the done channel has a reieveiPayLoad written to it when received.
}

type MultiCallResult[R any] struct {
	Result     any
	ReceiverID string
	Error      error
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
				if !c.permanent {
					RemoveReverseClient(r, cid)
				}
				//!! zlog.Info("zrpc: Unresponsive client from allReverseClients", c.permanent, cid, c.LastPolled, str, r.allReverseClients.Count())
			}
		})
	})
	return r
}

// NewReverseClient adds a ReverseClient to allReverseClients.
// When overriding HandleNewReverseReceiverFunc, you probably call this anyway, but use the ReverseClient.
func NewReverseClient(r *ReverseClienter, receiverID string, userAuthToken string, permanent bool) *ReverseClient {
	// zlog.Warn("NewReverseClient", receiverID)
	c := &ReverseClient{}
	c.rid = receiverID
	c.TimeoutSecs = 100
	c.LastPolled = time.Now()
	c.ClientUserToken = userAuthToken
	c.permanent = permanent
	c.pendingCalls = make(chan pendingCall, 100)
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
	// revCount := rand.Int31n(100)
	// zlog.Warn("ReversePoll1", revCount)
	// r.allReverseClients.ForAll(func(id string, rc *ReverseClient) {
	// 	zlog.Warn("ReversePoll findOrAddReverseClient:", id)
	// })
	rc := r.findOrAddReverseClient(receiverID, ci)
	// zlog.Warn("ReversePoll", receiverID, r.allReverseClients.Count(), rc.pendingCallsSent.Count(), "runc:", revCount)
	ticker := time.NewTicker(time.Duration(PollRestartSecs-1) * time.Second)
	select {
	case <-ticker.C:
		// zlog.Info(nil, "zrpc.ReversePoll ended without job, restarting", "runc:", revCount)
		return nil
	case call := <-rc.pendingCalls:
		if time.Since(call.Expires) >= 0 {
			var rp clientReceivePayload
			rp.TransportError = TransportError(zstr.Spaced("zrpc.ReverseCall timed out before being polled from executor", call.Method, call.Expires))
			call.done <- &rp
			break
		}
		zlog.Info(EnableLogReverseClient, "zrpc.ReversePoll added call:", call.Method)
		*cp = call.CallPayload
		// zlog.Warn("ReversePoll.Call:", call.CallPayload.Method, rc.rid, revCount)
		rc.pendingCallsSent.Set(call.Token, call)
	}
	ticker.Stop() // Very important to stop ticker, or memory leak
	// zlog.Warn("ReversePoll done", revCount)
	return nil
}

func (r *ReverseClienter) ReversePushResult(rp ReverseResult) error {
	// zlog.Info("ReversePushResult:", rp.Token, rp.Error)
	rc := r.findOrAddReverseClient(rp.ReverseReceiverID, nil)
	if rc == nil {
		return zlog.Error(rp.ReverseReceiverID)
	}
	pendingCall, got := rc.pendingCallsSent.Pop(rp.Token)
	// zlog.Info("zrpc.PushResult:", pendingCall.Method, got) // EnableLogReverseClient
	if !got {
		zlog.Error("No call for result with token:", rp.Token, "in", rp.ReverseReceiverID, rc.pendingCallsSent.Count()) // make some kind of transport error
		return NoCallForTokenErr
	}
	pendingCall.done <- &rp.clientReceivePayload
	return nil
}

func (r *ReverseClienter) findOrAddReverseClient(receiverID string, ci *ClientInfo) *ReverseClient {
	rc, _ := r.allReverseClients.Get(receiverID)
	if rc == nil {
		if ci == nil { // if ci is nil, it's from ReversePoll, don't add otherwise
			zlog.Error("findOrAddReverseClient ci=nil: no reverse client for id:", receiverID)
			return nil
		}
		// zlog.Warn("Add Rerverse Client:", receiverID, ci.Token)
		rc = NewReverseClient(r, receiverID, ci.Token, false)
		if r.HandleNewReverseReceiverFunc != nil {
			go r.HandleNewReverseReceiverFunc(receiverID, rc, ci)
		}
	}
	zlog.Assert(rc != nil, receiverID)
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
	// zlog.Warn("zrpc.RevCall:", method, rc.rid)
	var pc pendingCall
	pc.CallPayload = CallPayload{Method: method, Args: args}
	pc.placed = time.Now()
	pc.Expires = time.Now().Add(ztime.SecondsDur(timeoutSecs))
	token := zstr.GenerateRandomHexBytes(16)
	pc.CallPayload.Token = token
	pc.done = make(chan *clientReceivePayload, 10)
	rc.pendingCalls <- pc
	dur := ztime.SecondsDur(math.Min(timeoutSecs, PollRestartSecs))
	ticker := time.NewTicker(dur)
	select {
	case <-ticker.C:
		ticker.Stop() // Very important to stop ticker, or memory leak
		return zlog.Error("Reverse zrpc.Call timed out:", method, dur, rc.rid)
	case r := <-pc.done:
		// zlog.Warn("RevCall done", r.Error)
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

func CallAll[R any](r *ReverseClienter, timeoutSecs float64, method, idWildcard string, args any) []MultiCallResult[R] {
	var out []MultiCallResult[R]
	f, _ := args.(RowGetter)
	FuncForAll(r, idWildcard, method, func(receiverID string, rc *ReverseClient, i int) {
		var result R
		ts := timeoutSecs
		if ts == 0 {
			ts = rc.TimeoutSecs
		}
		a := args
		if f != nil {
			a = f(receiverID, i)
		}
		// zlog.Warn("Getter", i, a)
		err := rc.CallWithTimeout(ts, method, a, &result)
		m := MultiCallResult[R]{result, receiverID, err}
		out = append(out, m)
	})
	return out
}

func CallAllSimple(timeoutSecs float64, method, idWildcard string, args any) []error {
	var errs []error
	results := CallAll[Unused](nil, timeoutSecs, method, idWildcard, args)
	for _, r := range results {
		e := fmt.Errorf("%s: %w", r.ReceiverID, r.Error)
		errs = append(errs, e)
	}
	return errs
}

// FuncForAll itterates through all r.allReverseClients (which are typically open browsers sessions)
// If their id matches idWildcard, and there isn't already a call registered for callID,
// The do function is called with rc.ReverseClient, so the func can use the user's authentication token.
// It is used by CallAll() above to do a an rpc call to all.
func FuncForAll(r *ReverseClienter, idWildcard, callID string, do func(receiverID string, rc *ReverseClient, i int)) {
	if r == nil {
		r = MainReverseClienter
	}
	var wg sync.WaitGroup
	var i int
	// zlog.Info("FuncForAll:", r.allReverseClients.Count(), callID)
	r.allReverseClients.ForEach(func(id string, rc *ReverseClient) bool {
		// zlog.Info("FuncForAll2:", id, idWildcard)
		if idWildcard == "*" || zstr.MatchWildcard(idWildcard, id) {
			sid := id + ":" + callID
			if r.multiWaiting.Has(id) {
				return true
			}
			// zlog.Info("FuncForAll3:", id, callID)
			r.multiWaiting.Set(sid, true)
			wg.Add(1)
			go func(i int, sid string, rc *ReverseClient) {
				// zlog.Info("FuncForAll4:", sid)
				do(id, rc, i)
				r.multiWaiting.Remove(sid)
				wg.Done()
			}(i, sid, rc)
			i++
		}
		return true
	})
	wg.Wait()
}
