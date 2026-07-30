package zwebrtc

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
	"github.com/torlangballe/zutil/zdesktop"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zmap"
)

type WindowCaptureTarget struct {
	Title    string
	AppID    string
	CropRect zgeo.Rect
	DestSize zgeo.Size
}

type WindowWebRTCStreamer struct {
	streamMu sync.RWMutex
	Stream   zdesktop.WindowCaptureStream
	Video    *webrtc.TrackLocalStaticSample
	Audio    *webrtc.TrackLocalStaticSample
	HasAudio bool
	TargetID string
	Target   WindowCaptureTarget

	stop        chan struct{}
	stopOnce    sync.Once
	wg          sync.WaitGroup
	lastVideoTS time.Duration
	lastAudioTS time.Duration
}

type WebRTCPublishSession struct {
	PeerConnection *webrtc.PeerConnection
	Streamer       *WindowWebRTCStreamer
	sourceKey      string
	closeOnce      sync.Once
}

type sharedWindowSource struct {
	streamer *WindowWebRTCStreamer
	refs     int
}

var sharedWindowSources zmap.LockMap[string, *sharedWindowSource]

const (
	webrtcGatherTimeout       = 8 * time.Second
	videoStallRestartAfter    = 2 * time.Second
	videoStallRestartInterval = 1 * time.Second
)

var CreateWindowCaptureTargetFromIDFunc = func(targetID string) (WindowCaptureTarget, error) {
	return WindowCaptureTarget{
		Title:    "Google",
		AppID:    "com.google.Chrome",
		CropRect: zgeo.RectFromXY2(10, 10, 640, 480),
		DestSize: zgeo.SizeD(320, 240),
	}, nil
}

func (s *WebRTCPublishSession) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.PeerConnection != nil {
			_ = s.PeerConnection.Close()
		}
		if s.sourceKey != "" {
			releaseSharedWindowStreamer(s.sourceKey)
			s.sourceKey = ""
		}
	})
}

func sharedWindowSourceKey(targetID string, captureAudio bool) string {
	return fmt.Sprintf("%s|%t", targetID, captureAudio)
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

func releaseSharedWindowStreamer(key string) {
	if key == "" {
		return
	}
	var streamer *WindowWebRTCStreamer
	src, ok := sharedWindowSources.Get(key)
	if ok {
		src.refs--
		if src.refs <= 0 {
			streamer = src.streamer
			sharedWindowSources.Remove(key)
		}
	}
	if streamer != nil {
		streamer.Stop()
	}
}

func CreateWindowWebRTCPublisherFromOffer(remoteOffer webrtc.SessionDescription, targetID string, captureAudio bool, config webrtc.Configuration) (*WebRTCPublishSession, webrtc.SessionDescription, error) {
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
	session := &WebRTCPublishSession{PeerConnection: peerConnection, Streamer: streamer, sourceKey: sourceKey}

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
	stream, err := zdesktop.StartWindowCaptureStreamWithCropAndDestSize(target.Title, target.AppID, captureAudio, target.CropRect, target.DestSize)
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
		zdesktop.StopWindowCaptureStream(stream)
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
			zdesktop.StopWindowCaptureStream(stream)
			return nil, err
		}
	}
	return &WindowWebRTCStreamer{
		Stream:   stream,
		Video:    videoTrack,
		Audio:    audioTrack,
		HasAudio: captureAudio,
		Target:   target,
		stop:     make(chan struct{}),
	}, nil
}

func (s *WindowWebRTCStreamer) currentStream() zdesktop.WindowCaptureStream {
	return s.Stream
}

func (s *WindowWebRTCStreamer) currentTarget() WindowCaptureTarget {
	return s.Target
}

func (s *WindowWebRTCStreamer) restartTargetForCurrentID() (WindowCaptureTarget, error) {
	if s != nil && s.TargetID != "" {
		t, err := CreateWindowCaptureTargetFromIDFunc(s.TargetID)
		if err != nil {
			return WindowCaptureTarget{}, err
		}
		return t, nil
	}
	return WindowCaptureTarget{}, errors.New("no target ID")
}

func (s *WindowWebRTCStreamer) swapStream(newStream zdesktop.WindowCaptureStream, newTarget WindowCaptureTarget) zdesktop.WindowCaptureStream {
	s.streamMu.Lock()
	old := s.Stream
	s.Stream = newStream
	s.Target = newTarget
	s.lastVideoTS = 0
	s.lastAudioTS = 0
	s.streamMu.Unlock()
	return old
}

// RestartWindowTarget swaps the native capture source while keeping the existing
// WebRTC tracks/session alive.
func (s *WindowWebRTCStreamer) RestartWindowTarget(target WindowCaptureTarget) error {
	if s == nil {
		return errors.New("streamer is nil")
	}
	newStream, err := zdesktop.StartWindowCaptureStreamWithCropAndDestSize(target.Title, target.AppID, s.HasAudio, target.CropRect, target.DestSize)
	if err != nil {
		return err
	}
	old := s.swapStream(newStream, target)
	zdesktop.StopWindowCaptureStream(old)
	return nil
}

func (s *WindowWebRTCStreamer) AddTracks(peerConnection *webrtc.PeerConnection) error {
	if s == nil || peerConnection == nil {
		return errors.New("streamer or peer connection is nil")
	}
	videoSender, err := peerConnection.AddTrack(s.Video)
	if err != nil {
		return err
	}
	var audioSender *webrtc.RTPSender
	if s.HasAudio && s.Audio != nil {
		audioSender, err = peerConnection.AddTrack(s.Audio)
		if err != nil {
			return err
		}
	}
	consumeRTCP := func(sender *webrtc.RTPSender) {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rerr := sender.Read(rtcpBuf); rerr != nil {
				return
			}
		}
	}
	go consumeRTCP(videoSender)
	if audioSender != nil {
		go consumeRTCP(audioSender)
	}
	return nil
}

func (s *WindowWebRTCStreamer) Start() {
	if s == nil {
		return
	}
	s.wg.Add(2)
	go s.runVideoLoop()
	if s.HasAudio {
		go s.runAudioLoop()
		return
	}
	s.wg.Done()
}

func (s *WindowWebRTCStreamer) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		close(s.stop)
		s.wg.Wait()
		zdesktop.StopWindowCaptureStream(s.currentStream())
	})
}

func (s *WebRTCPublishSession) RestartWindowTarget(targetID string) error {
	if s == nil || s.Streamer == nil {
		return errors.New("session or streamer is nil")
	}
	target, err := CreateWindowCaptureTargetFromIDFunc(targetID)
	if err != nil {
		return err
	}
	return s.Streamer.RestartWindowTarget(target)
}

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
		streamRunning := zdesktop.IsWindowCaptureStreamRunning(stream)
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
		sample, ok := zdesktop.CaptureWindowStreamH264Sample(s.currentStream())
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
		sample, ok := zdesktop.CaptureWindowStreamOpusSample(s.currentStream())
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
