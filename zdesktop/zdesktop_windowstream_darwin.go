//go:build !linux

package zdesktop

// #cgo LDFLAGS: -framework CoreVideo
// #cgo LDFLAGS: -framework Foundation
// #cgo LDFLAGS: -framework AppKit
// #cgo LDFLAGS: -framework CoreGraphics
// #cgo LDFLAGS: -framework AVFoundation
// #cgo LDFLAGS: -framework CoreMedia
// #cgo LDFLAGS: -framework ScreenCaptureKit
// #cgo LDFLAGS: -framework VideoToolbox
// #cgo LDFLAGS: -framework AudioToolbox
// #include <stdlib.h>
// #include "zdesktop_windowstream_darwin.h"
/*
int copyLatestWindowStreamAudioOpus(void *stream, unsigned char **data, int *byteLength, int *sampleRate, int *channels, int64_t *ptsNS);
*/
import "C"

import (
	"errors"
	"fmt"
	"io"
	"time"
	"unsafe"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zwebrtc"
)

type WindowH264Sample struct {
	Data []byte
	PTS  time.Duration
}

type WindowOpusSample struct {
	Data       []byte
	SampleRate int
	Channels   int
	Duration   time.Duration
	PTS        time.Duration
}

func StartWindowCaptureStream(winTitle, appBundleID string, captureAudio bool) (WindowCaptureStream, error) {
	return StartWindowCaptureStreamWithCropAndDestSize(winTitle, appBundleID, captureAudio, zgeo.Rect{}, zgeo.Size{})
}

func StartWindowCaptureStreamWithCropAndDestSize(winTitle, appBundleID string, captureAudio bool, cropRect zgeo.Rect, destSize zgeo.Size) (WindowCaptureStream, error) {
	if !CanRecordScreen() {
		return nil, errors.New("screen recording permission is not granted")
	}
	zlog.Info("StartWindowCaptureStreamWithCropAndDestSize:", captureAudio)
	if captureAudio && !CanUseMicrophone() {
		return nil, errors.New("audio recording permission is not granted")
	}

	C.EnsureCocoaGraphicsInitialized()

	cTitle := C.CString(winTitle)
	cApp := C.CString(appBundleID)
	defer C.free(unsafe.Pointer(cTitle))
	defer C.free(unsafe.Pointer(cApp))

	errBufLen := 1024
	errBuf := C.malloc(C.size_t(errBufLen))
	defer C.free(errBuf)
	*((*C.char)(errBuf)) = 0

	audio := C.int(0)
	if captureAudio {
		audio = 1
	}
	crop := C.CGRect{}
	crop.origin.x = C.CGFloat(cropRect.Pos.X)
	crop.origin.y = C.CGFloat(cropRect.Pos.Y)
	crop.size.width = C.CGFloat(cropRect.Size.W)
	crop.size.height = C.CGFloat(cropRect.Size.H)

	dest := C.CGSize{}
	dest.width = C.CGFloat(destSize.W)
	dest.height = C.CGFloat(destSize.H)

	stream := C.startWindowCaptureStreamWithCropAndDestSize(cTitle, cApp, audio, crop, dest, (*C.char)(errBuf), C.int(errBufLen))
	if stream == nil {
		errText := C.GoString((*C.char)(errBuf))
		if errText == "" {
			errText = "failed to start window capture stream"
		}
		return nil, errors.New(errText)
	}
	return WindowCaptureStream(stream), nil
}

func StopWindowCaptureStream(stream WindowCaptureStream) {
	if stream == nil {
		return
	}
	C.stopWindowCaptureStream(unsafe.Pointer(stream))
}

func IsWindowCaptureStreamRunning(stream WindowCaptureStream) bool {
	if stream == nil {
		return false
	}
	return C.isWindowCaptureStreamRunning(unsafe.Pointer(stream)) != 0
}

func CaptureWindowStreamH264Sample(stream WindowCaptureStream) (WindowH264Sample, bool) {
	if stream == nil {
		return WindowH264Sample{}, false
	}
	var ptr *C.uchar
	var length C.int
	var ptsNS C.int64_t
	ok := C.copyLatestWindowStreamH264(unsafe.Pointer(stream), &ptr, &length, &ptsNS)
	if ok == 0 || ptr == nil || length <= 0 {
		return WindowH264Sample{}, false
	}
	defer C.free(unsafe.Pointer(ptr))
	data := C.GoBytes(unsafe.Pointer(ptr), length)
	return WindowH264Sample{Data: data, PTS: time.Duration(int64(ptsNS))}, true
}

func CaptureWindowStreamOpusSample(stream WindowCaptureStream) (WindowOpusSample, bool) {
	if stream == nil {
		return WindowOpusSample{}, false
	}
	var ptr *C.uchar
	var byteLength C.int
	var sampleRate C.int
	var channels C.int
	var ptsNS C.int64_t
	ok := C.copyLatestWindowStreamAudioOpus(unsafe.Pointer(stream), &ptr, &byteLength, &sampleRate, &channels, &ptsNS)
	if ok == 0 || ptr == nil || byteLength <= 0 || sampleRate <= 0 || channels <= 0 {
		return WindowOpusSample{}, false
	}
	defer C.free(unsafe.Pointer(ptr))
	data := C.GoBytes(unsafe.Pointer(ptr), byteLength)

	return WindowOpusSample{
		Data:       data,
		SampleRate: int(sampleRate),
		Channels:   int(channels),
		Duration:   0,
		PTS:        time.Duration(int64(ptsNS)),
	}, true
}

func acquireSharedWindowStreamer(targetID string, captureAudio bool) (*WindowWebRTCStreamer, string, error) {
	key := sharedWindowSourceKey(targetID, captureAudio)
	src, ok := sharedWindowSources.Get(key)
	if ok {
		src.refs++
		streamer := src.streamer
		return streamer, key, nil
	}
	target, err := CreateWindowCaptureTargetFromIDFunc(targetID)
	if err != nil {
		return nil, "", err
	}
	streamer, err := NewWindowWebRTCStreamer(target, captureAudio)
	// zlog.Info("createWebRTCPublisherFromOffer:", target, captureAudio, err)
	if err != nil {
		return nil, "", err
	}
	streamer.TargetID = targetID
	streamer.Start()

	// sharedWindowSources.mu.Lock()
	src, ok = sharedWindowSources.Get(key)
	if ok {
		src.refs++
		// sharedWindowSources.mu.Unlock()
		streamer.Stop()
		return src.streamer, key, nil
	}
	sharedWindowSources.Set(key, &sharedWindowSource{streamer: streamer, refs: 1})
	// sharedWindowSources.mu.Unlock()
	return streamer, key, nil
}

func CreateWindowWebRTCPublisherFromOffer(remoteOffer webrtc.SessionDescription, targetID string, captureAudio bool, config webrtc.Configuration) (*zwebrtc.WebRTCPublishSession, webrtc.SessionDescription, error) {
	zlog.Info("CreateWindowWebRTCPublisherFromOffer:", targetID, captureAudio)
	peerConnection, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, webrtc.SessionDescription{}, err
	}
	streamer, sourceKey, err := acquireSharedWindowStreamer(targetID, captureAudio)
	if err != nil {
		_ = peerConnection.Close()
		return nil, webrtc.SessionDescription{}, err
	}
	session := &zwebrtc.WebRTCPublishSession{PeerConnection: peerConnection, Streamer: streamer, SourceKey: sourceKey}
	session.CloseFunc = func() error {
		CloseWebRTCPublishSession(session)
		return nil
	}
	if err = peerConnection.SetRemoteDescription(remoteOffer); err != nil {
		session.Close()
		return nil, webrtc.SessionDescription{}, err
	}
	if err = streamer.AddTracks(peerConnection); err != nil {
		session.Close()
		return nil, webrtc.SessionDescription{}, err
	}
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		session.Close()
		return nil, webrtc.SessionDescription{}, err
	}
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)
	if err = peerConnection.SetLocalDescription(answer); err != nil {
		session.Close()
		return nil, webrtc.SessionDescription{}, err
	}
	select {
	case <-gatherComplete:
	case <-time.After(webrtcGatherTimeout):
		session.Close()
		zlog.Info("createWebRTCPublisherFromOffer timed out waiting for ICE gathering after", webrtcGatherTimeout)
		return nil, webrtc.SessionDescription{}, fmt.Errorf("timed out waiting for ICE gathering after %s", webrtcGatherTimeout)
	}
	if peerConnection.LocalDescription() == nil {
		session.Close()
		return nil, webrtc.SessionDescription{}, errors.New("failed to gather local description")
	}

	peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
			session.Close()
		}
	})

	return session, *peerConnection.LocalDescription(), nil
}

func NewWindowWebRTCStreamer(target WindowCaptureTarget, captureAudio bool) (*WindowWebRTCStreamer, error) {
	captureAudio = true
	stream, err := StartWindowCaptureStreamWithCropAndDestSize(target.Title, target.AppID, captureAudio, target.CropRect, target.DestSize)
	zlog.Info("NewWindowWebRTCStreamer1:", target, err)
	if err != nil {
		return nil, err
	}
	videoTrack, err := webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264, ClockRate: 90000},
		"video",
		"window",
	)
	if err != nil {
		StopWindowCaptureStream(stream)
		return nil, err
	}
	var audioTrack *webrtc.TrackLocalStaticSample
	if captureAudio {
		audioTrack, err = webrtc.NewTrackLocalStaticSample(
			webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus, ClockRate: 48000, Channels: 2, SDPFmtpLine: "minptime=10;useinbandfec=1;stereo=1;sprop-stereo=1"},
			"audio",
			"window",
		)
		if err != nil {
			StopWindowCaptureStream(stream)
			return nil, err
		}
	}
	return &WindowWebRTCStreamer{
		Stream:   stream,
		Video:    videoTrack,
		Audio:    audioTrack,
		HasAudio: true, //captureAudio,
		Target:   target,
		stop:     make(chan struct{}),
	}, nil
}

// func (s *WebRTCPublishSession) RestartWindowTarget(targetID string) error {
// 	if s == nil || s.Streamer == nil {
// 		return errors.New("session or streamer is nil")
// 	}
// 	target, err := CreateWindowCaptureTargetFromIDFunc(targetID)
// 	if err != nil {
// 		return err
// 	}
// 	return s.Streamer.RestartWindowTarget(target)
// }

func (s *WindowWebRTCStreamer) runVideoLoop() {
	defer s.wg.Done()
	var stallSince time.Time
	var lastRestartAttempt time.Time
	var lastProgressPTS time.Duration

	markStalled := func() {
		now := time.Now()
		if stallSince.IsZero() {
			stallSince = now
		}
		stream := s.currentStream()
		streamRunning := IsWindowCaptureStreamRunning(stream)
		if (now.Sub(stallSince) >= videoStallRestartAfter || !streamRunning) && now.Sub(lastRestartAttempt) >= videoStallRestartInterval {
			lastRestartAttempt = now
			target, err := s.restartTargetForCurrentID()
			if err != nil {
				zlog.Info("WindowWebRTCStreamer: stalled video, failed to restart target:", err)
				return
			}
			if err := s.RestartWindowTarget(target); err != nil {
				zlog.Info("WindowWebRTCStreamer: stalled video, restart failed:", target, err)
			} else {
				zlog.Info("WindowWebRTCStreamer: recovered stalled video by restarting target:", target)
				stallSince = time.Time{}
				lastProgressPTS = 0
			}
		}
	}

	for {
		select {
		case <-s.stop:
			return
		default:
		}
		sample, ok := CaptureWindowStreamH264Sample(s.currentStream())
		if !ok {
			markStalled()
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if len(sample.Data) == 0 || (lastProgressPTS != 0 && sample.PTS <= lastProgressPTS) {
			markStalled()
			time.Sleep(20 * time.Millisecond)
			continue
		}
		stallSince = time.Time{}
		lastProgressPTS = sample.PTS
		duration := 33 * time.Millisecond
		if s.lastVideoTS != 0 {
			delta := sample.PTS - s.lastVideoTS
			if delta > 0 && delta < time.Second {
				duration = delta
			}
		}
		s.lastVideoTS = sample.PTS
		if err := s.Video.WriteSample(media.Sample{Data: sample.Data, Duration: duration}); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				return
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
	}
}

func (s *WindowWebRTCStreamer) runAudioLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stop:
			return
		default:
		}
		sample, ok := CaptureWindowStreamOpusSample(s.currentStream())
		if !ok {
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if len(sample.Data) == 0 {
			continue
		}
		d := sample.Duration
		if d <= 0 && s.lastAudioTS != 0 {
			delta := sample.PTS - s.lastAudioTS
			if delta > 0 && delta < time.Second {
				d = delta
			}
		}
		if d <= 0 {
			d = 20 * time.Millisecond
		}
		s.lastAudioTS = sample.PTS
		if err := s.Audio.WriteSample(media.Sample{Data: sample.Data, Duration: d}); err != nil {
			if errors.Is(err, io.ErrClosedPipe) {
				return
			}
			time.Sleep(10 * time.Millisecond)
			continue
		}
	}
}
