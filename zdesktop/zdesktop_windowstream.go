//go:build !linux

package zdesktop

// darwin in go:build is temporary

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/pion/webrtc/v3"
	"github.com/torlangballe/zutil/zgeo"
	"github.com/torlangballe/zutil/zmap"
	"github.com/torlangballe/zutil/zwebrtc"
)

type WindowCaptureStream unsafe.Pointer

type WindowWebRTCStreamer struct {
	streamMu sync.RWMutex
	Stream   WindowCaptureStream
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

type WindowCaptureTarget struct {
	Title    string
	AppID    string
	CropRect zgeo.Rect
	DestSize zgeo.Size
}

var CreateWindowCaptureTargetFromIDFunc = func(targetID string) (WindowCaptureTarget, error) {
	return WindowCaptureTarget{
		Title:    "Google",
		AppID:    "com.google.Chrome",
		CropRect: zgeo.RectFromXY2(10, 10, 640, 480),
		DestSize: zgeo.SizeD(320, 240),
	}, nil
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

// RestartWindowTarget swaps the native capture source while keeping the existing
// WebRTC tracks/session alive.
func (s *WindowWebRTCStreamer) RestartWindowTarget(target WindowCaptureTarget) error {
	if s == nil {
		return errors.New("streamer is nil")
	}
	newStream, err := StartWindowCaptureStreamWithCropAndDestSize(target.Title, target.AppID, s.HasAudio, target.CropRect, target.DestSize)
	if err != nil {
		return err
	}
	old := s.swapStream(newStream, target)
	StopWindowCaptureStream(old)
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
		StopWindowCaptureStream(s.currentStream())
	})
}

type Stopper interface {
	Stop()
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

func CloseWebRTCPublishSession(s *zwebrtc.WebRTCPublishSession) {
	if s == nil {
		return
	}
	s.CloseOnce.Do(func() {
		if s.PeerConnection != nil {
			_ = s.PeerConnection.Close()
		}
		if s.SourceKey != "" {
			releaseSharedWindowStreamer(s.SourceKey)
			s.SourceKey = ""
		}
	})
}

func sharedWindowSourceKey(targetID string, captureAudio bool) string {
	return fmt.Sprintf("%s|%t", targetID, captureAudio)
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
		var a any
		a = streamer
		if stopper, ok := a.(Stopper); ok {
			stopper.Stop()
		}
	}
}

func (s *WindowWebRTCStreamer) currentStream() WindowCaptureStream {
	return s.Stream
}

func (s *WindowWebRTCStreamer) currentTarget() WindowCaptureTarget {
	return s.Target
}

func (s *WindowWebRTCStreamer) swapStream(newStream WindowCaptureStream, newTarget WindowCaptureTarget) WindowCaptureStream {
	s.streamMu.Lock()
	old := s.Stream
	s.Stream = newStream
	s.Target = newTarget
	s.lastVideoTS = 0
	s.lastAudioTS = 0
	s.streamMu.Unlock()
	return old
}
