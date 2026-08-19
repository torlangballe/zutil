//go:build js && zui

package zwebrtc

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"
	"time"

	"github.com/torlangballe/zui/zcontainer"
	"github.com/torlangballe/zui/zdom"
	"github.com/torlangballe/zui/zview"
	"github.com/torlangballe/zutil/xrpc"
	"github.com/torlangballe/zutil/zfile"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zhttp"
	"github.com/torlangballe/zutil/zkeyvalue"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zstr"
	"github.com/torlangballe/zutil/ztime"
	"github.com/torlangballe/zutil/ztimer"
)

type WebRTCView struct {
	zcontainer.ContainerView
	BaseURL   string
	OfferPath string
	ClosePath string
	IceURLs   []string
	RPCCaller xrpc.Callable

	pc                     js.Value
	remoteStream           js.Value
	sessionID              string
	playAttemptTimerID     js.Value
	playbackHealthTimer    *ztimer.Repeater
	videoFrameFn           js.Func
	videoFrameRequestID    int
	lastVideoFrameAtMs     int64
	hadVideoFrameProgress  bool
	lastPresentedFrames    float64
	playInProgress         bool
	sampleTimer            *ztimer.Repeater
	runtimeFailureReported bool
	onTrackFn              js.Func
	onConnFn               js.Func
	onBeforeUnloadFn       js.Func

	TargetID string
	UseAudio bool

	HandleErrorFunc func(err error)
}

const (
	sessionStorageKey     = "zwebrtc.SessionID"
	initialVideoTimeoutMs = 7000
	runtimeStallTimeoutMs = 3000
)

func NewView(size zgeo.Size) *WebRTCView {
	v := &WebRTCView{
		pc:                 js.Null(),
		remoteStream:       js.Null(),
		playAttemptTimerID: js.Null(),
		// playHealthTimerID:  js.Null(),
		// playHealthFn:       js.Func{},
		videoFrameFn:     js.Func{},
		onTrackFn:        js.Func{},
		onConnFn:         js.Func{},
		onBeforeUnloadFn: js.Func{},
	}
	v.Init(v, size)
	return v
}

func (v *WebRTCView) Init(view zview.View, size zgeo.Size) {
	v.ContainerView.Init(v, "webrtc#type:video")
	v.SetMinSize(size)
	v.SetObjectName("video-input")
	v.Element.Set("autoplay", true)
	v.Element.Set("playsInline", true)
	v.Element.Set("muted", true)
	// v.Element.Set("controls", true)
	if v.OfferPath == "" {
		v.OfferPath = "/webrtc/offer"
	}
	if v.ClosePath == "" {
		v.ClosePath = "/webrtc/close"
	}
	if len(v.IceURLs) == 0 {
		v.IceURLs = []string{"stun:stun.l.google.com:19302"}
	}

	if false {
		if !v.onBeforeUnloadFn.IsUndefined() {
			v.onBeforeUnloadFn.Release()
		}
		v.onBeforeUnloadFn = js.FuncOf(func(this js.Value, args []js.Value) any {
			if v.sessionID != "" {
				if v.BaseURL == "" {
					return nil
				}
				closeURL := zfile.JoinPathParts(v.BaseURL, v.ClosePath)
				payload, _ := json.Marshal(CloseRequest{SessionID: v.sessionID})
				blob := js.Global().Get("Blob").New([]any{string(payload)}, map[string]any{"type": "application/json"})
				js.Global().Get("navigator").Call("sendBeacon", closeURL, blob)
				zkeyvalue.DefaultSessionStore.SetString(v.sessionID, sessionStorageKey, true)

				// js.Global().Get("sessionStorage").Call("removeItem", sessionStorageKey)
			}
			return nil
		})
		js.Global().Call("addEventListener", "beforeunload", v.onBeforeUnloadFn)
	}
	v.cleanupStaleSessionOnLoad()
	v.playbackHealthTimer = ztimer.NewRepeater()
	v.AddOnRemoveFunc(v.playbackHealthTimer.Stop)
	v.sampleTimer = ztimer.NewRepeater()
	v.AddOnRemoveFunc(v.sampleTimer.Stop)
}

func (v *WebRTCView) ReadyToShow(beforeWindow bool) {
	if beforeWindow {
		return
	}
	if v.RPCCaller != nil {
		v.AddOnRemoveFunc(func() {
			zlog.Info("WebRTCView: remove triggered")
			err := v.closeServerSession(v.sessionID)
			if err != nil {
				zlog.Error("Failed to close WebRTC session:", err)
			}
		})
	}
}

func (v *WebRTCView) cleanupStaleSessionOnLoad() {
	if v.BaseURL == "" {
		return
	}
	// store := js.Global().Get("sessionStorage")
	// if !store.Truthy() {
	// 	return
	// }
	stale, _ := zkeyvalue.DefaultSessionStore.GetString(sessionStorageKey)
	// stale := store.Call("getItem", sessionStorageKey).String()
	if stale == "" {
		return
	}
	v.closeServerSession(stale)
	zkeyvalue.DefaultSessionStore.RemoveForKey(sessionStorageKey, true)
}

func (v *WebRTCView) persistSessionID() {
	if v.sessionID == "" {
		zkeyvalue.DefaultSessionStore.RemoveForKey(sessionStorageKey, true)
		return
	}
	zkeyvalue.DefaultSessionStore.SetString(v.sessionID, sessionStorageKey, true)
}

func (v *WebRTCView) clearPlayAttemptTimer() {
	if v.playAttemptTimerID.Truthy() {
		js.Global().Call("clearTimeout", v.playAttemptTimerID)
		v.playAttemptTimerID = js.Null()
	}
}

func (v *WebRTCView) clearPlayHealthTimer() {
	v.playbackHealthTimer.Stop()
	if v.videoFrameRequestID != 0 {
		cancelFn := v.Element.Get("cancelVideoFrameCallback")
		if cancelFn.Type() == js.TypeFunction {
			v.Element.Call("cancelVideoFrameCallback", v.videoFrameRequestID)
		}
		v.videoFrameRequestID = 0
	}
	if !v.videoFrameFn.IsUndefined() {
		v.videoFrameFn.Release()
		v.videoFrameFn = js.Func{}
	}
}

func (v *WebRTCView) schedulePlayRetry(reason string, delayMs int) {
	v.clearPlayAttemptTimer()
	var retry js.Func
	retry = js.FuncOf(func(this js.Value, args []js.Value) any {
		v.playAttemptTimerID = js.Null()
		retry.Release()
		v.ensureVideoPlaying(reason)
		return nil
	})
	v.playAttemptTimerID = js.Global().Call("setTimeout", retry, delayMs)
}

func (v *WebRTCView) ensureVideoPlaying(reason string) {
	zlog.Info("ensureVideoPlaying:", reason, v.playInProgress)
	if !v.Element.Get("srcObject").Truthy() {
		return
	}
	if v.playInProgress {
		// v.playRequestedWhileBusy = true
		return
	}

	v.playInProgress = true
	playPromise := v.Element.Call("play")
	zdom.Resolve(playPromise, func(pval js.Value, err error) {
		v.playInProgress = false
		if err != nil {
			zlog.Info("ensureVideoPlaying play fail:", err)
			name := pval.Get("name").String()
			msg := pval.Get("message").String()
			if name == "AbortError" || msg == "play timeout" {
				v.schedulePlayRetry("retry-after-abort", 120)
			}
		}
	})
}

func (v *WebRTCView) attachRemoteStreamIfNeeded(stream js.Value, reason string, shouldAttemptPlay bool) {
	zlog.Info("attachRemoteStreamIfNeeded:", reason, stream.Truthy(), v.Element.Get("srcObject").Truthy())
	if !stream.Truthy() {
		return
	}
	if !v.Element.Get("srcObject").Equal(stream) {
		v.Element.Set("srcObject", stream)
	}
	if shouldAttemptPlay {
		v.ensureVideoPlaying(reason)
	}
}

func (v *WebRTCView) hasLiveVideoTrack() bool {
	if !v.remoteStream.Truthy() {
		return false
	}
	tracks := v.remoteStream.Call("getVideoTracks")
	if !tracks.Truthy() {
		return false
	}
	for i := 0; i < tracks.Get("length").Int(); i++ {
		track := tracks.Index(i)
		if !track.Truthy() {
			continue
		}
		if track.Get("readyState").String() == "live" && !track.Get("muted").Bool() {
			return true
		}
	}
	return false
}

func (v *WebRTCView) isVideoReady() bool {
	if !v.Element.Get("srcObject").Truthy() {
		return false
	}
	if v.Element.Get("readyState").Int() < 2 {
		return false
	}
	return v.hasLiveVideoTrack()
}

func (v *WebRTCView) waitForInitialVideo(timeoutMs int) error {
	if v.isVideoReady() {
		return nil
	}

	done := make(chan error, 1)
	finished := false
	finish := func(err error) {
		if finished {
			return
		}
		finished = true
		done <- err
	}

	var tickFn js.Func
	var timeoutFn js.Func
	var intervalID js.Value
	var timeoutID js.Value

	tickFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !v.pc.Truthy() {
			finish(errors.New("peer connection closed before video became ready"))
			return nil
		}
		state := v.pc.Get("connectionState").String()
		switch state {
		case "failed", "closed", "disconnected":
			finish(fmt.Errorf("peer connection state is %s", state))
			return nil
		}
		if v.isVideoReady() {
			finish(nil)
		}
		return nil
	})

	timeoutFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		finish(fmt.Errorf("timeout waiting for video frames (%d ms)", timeoutMs))
		return nil
	})

	intervalID = js.Global().Call("setInterval", tickFn, 120)
	timeoutID = js.Global().Call("setTimeout", timeoutFn, timeoutMs)
	err := <-done

	js.Global().Call("clearInterval", intervalID)
	js.Global().Call("clearTimeout", timeoutID)
	tickFn.Release()
	timeoutFn.Release()
	return err
}

func (v *WebRTCView) failRuntimePlayback(reason string) {
	if v.runtimeFailureReported {
		return
	}
	v.runtimeFailureReported = true
	zlog.Info("WebRTC runtime playback failed:", reason, "session=", v.sessionID)
	if v.HandleErrorFunc != nil {
		v.HandleErrorFunc(errors.New(reason))
	}
}

func (v *WebRTCView) scheduleNextVideoFrameCallback() {
	if !v.pc.Truthy() || !v.Element.Get("srcObject").Truthy() {
		return
	}
	requestFn := v.Element.Get("requestVideoFrameCallback")
	if requestFn.Type() != js.TypeFunction {
		return
	}
	v.videoFrameRequestID = v.Element.Call("requestVideoFrameCallback", v.videoFrameFn).Int()
}

func (v *WebRTCView) startVideoFrameWatch() {
	v.videoFrameRequestID = 0
	v.hadVideoFrameProgress = false
	v.lastVideoFrameAtMs = ztime.GetRuntimeMillisecs()
	v.lastPresentedFrames = 0
	requestFn := v.Element.Get("requestVideoFrameCallback")
	if requestFn.Type() != js.TypeFunction {
		zlog.Info("requestVideoFrameCallback unsupported")
		return
	}
	v.videoFrameFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 1 {
			metadata := args[1]
			presented := metadata.Get("presentedFrames")
			if presented.Type() == js.TypeNumber {
				count := presented.Float()
				if count > v.lastPresentedFrames {
					v.lastPresentedFrames = count
					v.lastVideoFrameAtMs = ztime.GetRuntimeMillisecs()
					v.hadVideoFrameProgress = true
				}
			}
		}
		v.scheduleNextVideoFrameCallback()
		return nil
	})
	v.scheduleNextVideoFrameCallback()
}

func (v *WebRTCView) startPlaybackHealthWatch() {
	v.playbackHealthTimer.Stop()
	v.runtimeFailureReported = false
	v.startVideoFrameWatch()

	v.playbackHealthTimer.Set(0.4, false, func() bool {
		if !v.pc.Truthy() {
			return true
		}
		if !v.hadVideoFrameProgress {
			return true
		}
		nowMs := ztime.GetRuntimeMillisecs()
		if nowMs-v.lastVideoFrameAtMs > runtimeStallTimeoutMs {
			v.failRuntimePlayback(fmt.Sprintf("lastVideoFrame stalled for >%dms", runtimeStallTimeoutMs))
		}
		return true
	})
}

func (v *WebRTCView) closeServerSession(id string) error {
	if id == "" {
		return nil
	}
	return v.RPCCaller.Call("WebRTCServer.Close", id, nil)
}

func (v *WebRTCView) StopPlayback() {
	zlog.Info("StopPlayback")
	v.clearPlayAttemptTimer()
	v.clearPlayHealthTimer()
	v.playInProgress = false
	v.videoFrameRequestID = 0
	v.lastVideoFrameAtMs = 0
	v.hadVideoFrameProgress = false
	v.lastPresentedFrames = 0
	v.runtimeFailureReported = false
	if v.pc.Truthy() {
		v.pc.Set("ontrack", js.Null())
		v.pc.Set("onconnectionstatechange", js.Null())
		v.pc.Call("close")
		v.pc = js.Null()
	}
	if !v.onTrackFn.IsUndefined() {
		v.onTrackFn.Release()
		v.onTrackFn = js.Func{}
	}
	if !v.onConnFn.IsUndefined() {
		v.onConnFn.Release()
		v.onConnFn = js.Func{}
	}
	v.remoteStream = js.Null()
	v.Element.Set("srcObject", js.Null())
	if v.sessionID != "" {
		v.closeServerSession(v.sessionID)
	}
	v.sessionID = ""
	v.persistSessionID()
}

func (v *WebRTCView) StartPlayback() {
	err := v.startPlayback()
	if err != nil && v.HandleErrorFunc != nil {
		v.HandleErrorFunc(err)
	}
}

func (v *WebRTCView) startPlayback() error {
	zlog.Info("WebRTCView StartPlayback: target=", v.TargetID, "audio=", v.UseAudio)
	v.StopPlayback()
	iceServer := map[string]any{"urls": zstr.StringsToAnySlice(v.IceURLs)}
	pcConfig := map[string]any{"iceServers": []any{iceServer}}
	// rtc := js.Global().Get("RTCPeerConnection")
	// v.pc = rtc.New(pcConfig)
	v.pc = zdom.New("RTCPeerConnection", pcConfig)
	v.remoteStream = zdom.New("MediaStream")
	v.attachRemoteStreamIfNeeded(v.remoteStream, "pc-created", false)
	v.onTrackFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 {
			return nil
		}
		event := args[0]
		streams := event.Get("streams")
		if streams.Truthy() && streams.Get("length").Int() > 0 {
			v.remoteStream = streams.Index(0)
			v.attachRemoteStreamIfNeeded(v.remoteStream, "ontrack-stream", true)
			return nil
		}
		track := event.Get("track")
		if track.Truthy() {
			v.remoteStream.Call("addTrack", track)
			var onUnmute js.Func
			onUnmute = js.FuncOf(func(this js.Value, args []js.Value) any {
				track.Set("onunmute", js.Null())
				onUnmute.Release()
				v.ensureVideoPlaying("track-unmute-" + track.Get("kind").String())
				return nil
			})
			track.Set("onunmute", onUnmute)
		}
		v.attachRemoteStreamIfNeeded(v.remoteStream, "ontrack", true)
		return nil
	})
	v.pc.Set("ontrack", v.onTrackFn)

	v.onConnFn = js.FuncOf(func(this js.Value, args []js.Value) any {
		state := v.pc.Get("connectionState").String()
		zlog.Info("pc state: new: ", state)
		return nil
	})
	v.pc.Set("onconnectionstatechange", v.onConnFn)
	v.pc.Call("addTransceiver", "video", js.ValueOf(map[string]any{"direction": "recvonly"}))
	if v.UseAudio {
		v.pc.Call("addTransceiver", "audio", js.ValueOf(map[string]any{"direction": "recvonly"}))
	}

	offer, err := zdom.ResolveInPlace(v.pc.Call("createOffer"))
	if err != nil {
		return fmt.Errorf("createOffer failed: %w", err)
	}
	_, err = zdom.ResolveInPlace(v.pc.Call("setLocalDescription", offer))
	if err != nil {
		return fmt.Errorf("setLocalDescription failed: %w", err)
	}

	offerURL := zfile.JoinPathParts(v.BaseURL, v.OfferPath)
	offerLocal := v.pc.Get("localDescription")

	offerJSON := zdom.ObjectToJSONString(offerLocal)
	offerMap := map[string]any{}
	if err := json.Unmarshal([]byte(offerJSON), &offerMap); err != nil {
		return fmt.Errorf("unmarshal offer failed: %w", err)
	}
	payload := map[string]any{
		"offer":        offerMap,
		"captureAudio": !v.IsMuted(),
		"target":       v.TargetID,
	}
	var resp OfferResponse
	params := zhttp.MakeParameters()
	_, err = zhttp.Post(offerURL, params, payload, &resp)
	v.sessionID = resp.SessionID
	v.persistSessionID()
	answerObj := js.ValueOf(map[string]any{
		"type": resp.Answer.Type.String(),
		"sdp":  resp.Answer.SDP,
	})
	_, err = zdom.ResolveInPlace(v.pc.Call("setRemoteDescription", answerObj))
	if err != nil {
		return fmt.Errorf("setRemoteDescription failed: %w", err)
	}
	v.ensureVideoPlaying("after-remote-description")
	err = v.waitForInitialVideo(initialVideoTimeoutMs)
	if err != nil {
		v.StopPlayback()
		return fmt.Errorf("video did not start: %w", err)
	}
	v.startPlaybackHealthWatch()
	return nil
}

func (v *WebRTCView) SetMuted(muted bool) {
	if v.Element.Truthy() {
		zlog.Info("SetMuted:", muted)
		v.Element.Set("muted", muted)
	}
}

func (v *WebRTCView) IsMuted() bool {
	if v.Element.Truthy() {
		return v.Element.Get("muted").Bool()
	}
	return false
}

func (v *WebRTCView) RepeatSampleLoudness(frequency time.Duration, got func(volume float64)) {
	v.sampleTimer.Stop()
	if got == nil || frequency == 0 {
		return
	}
	v.sampleTimer.Set(frequency.Seconds(), false, func() bool {
		if v.pc.IsUndefined() || v.pc.IsNull() {
			return true
		}
		stats, err := zdom.ResolveInPlace(v.pc.Call("getStats"))
		if err != nil {
			zlog.Error("getStats failed:", err)
			return true
		}
		found := new(bool)
		zdom.ForEach(stats, func(e js.Value) {
			if *found {
				return
			}
			if e.Get("type").String() != "inbound-rtp" {
				return
			}
			if e.Get("kind").String() != "audio" {
				return
			}
			audioLevelVal := e.Get("audioLevel")
			if !audioLevelVal.IsUndefined() && !audioLevelVal.IsNull() {
				*found = true
				got(audioLevelVal.Float())
			}
		})
		return true
	})
}

func main() {
	// Keep the Go WASM binary alive in the background
	select {}
}
