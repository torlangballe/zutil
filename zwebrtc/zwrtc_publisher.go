//go:build !js

package zwebrtc

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/pion/webrtc/v3"
	"github.com/torlangballe/zutil/zlog"
)

type WebRTCPublisher struct {
	cfg           webrtc.Configuration
	sessions      map[string]*WebRTCPublishSession
	nextSessionID uint64
	mu            sync.Mutex

	CreatePublisherFromOfferFunc func(remoteOffer webrtc.SessionDescription, targetID string, captureAudio bool, config webrtc.Configuration) (*WebRTCPublishSession, webrtc.SessionDescription, error)
}

type PublisherOfferArgs struct {
	Offer        webrtc.SessionDescription `json:"offer"`
	TargetID     string                    `json:"target,omitempty"`
	CaptureAudio bool                      `json:"captureAudio,omitempty"`
}

type PublisherOfferResult struct {
	PublisherSessionID string                    `json:"publisherSessionID,omitempty"`
	Answer             webrtc.SessionDescription `json:"answer,omitempty"`
}

type WebRTCPublishSession struct {
	PeerConnection *webrtc.PeerConnection

	Streamer  any // *zdesktop.WindowWebRTCStreamer
	CloseFunc func() error
	SourceKey string
	CloseOnce sync.Once
}

func NewWebRTCPublisher(cfg webrtc.Configuration) *WebRTCPublisher {
	return &WebRTCPublisher{
		cfg:      cfg,
		sessions: map[string]*WebRTCPublishSession{},
	}
}

func handleOffer(a *WebRTCPublisher, offer webrtc.SessionDescription, targetID string, captureAudio bool) (publisherSessionID string, answer webrtc.SessionDescription, err error) {
	if a == nil {
		return "", webrtc.SessionDescription{}, errors.New("window publisher agent is nil")
	}
	sess, answer, err := a.CreatePublisherFromOfferFunc(offer, targetID, captureAudio, a.cfg)
	if err != nil {
		return "", webrtc.SessionDescription{}, err
	}
	id := fmt.Sprintf("pub-%d", atomic.AddUint64(&a.nextSessionID, 1))
	a.mu.Lock()
	a.sessions[id] = sess
	a.mu.Unlock()
	return id, answer, nil
}

func (a *WebRTCPublisher) HandleClose(publisherSessionID string) error {
	if a == nil {
		return errors.New("window publisher agent is nil")
	}
	if publisherSessionID == "" {
		return nil
	}
	a.mu.Lock()
	sess, ok := a.sessions[publisherSessionID]
	if ok {
		delete(a.sessions, publisherSessionID)
	}
	a.mu.Unlock()
	if ok {
		if sess.CloseFunc != nil {
			sess.CloseFunc()
		}
	}
	return nil
}

func (a *WebRTCPublisher) CloseAll() {
	if a == nil {
		return
	}
	a.mu.Lock()
	sessions := a.sessions
	a.sessions = map[string]*WebRTCPublishSession{}
	a.mu.Unlock()
	for _, sess := range sessions {
		if sess.CloseFunc != nil {
			sess.CloseFunc()
		}
	}
}

func (a *WebRTCPublisher) Offer(args PublisherOfferArgs, result *PublisherOfferResult) error {
	if a == nil {
		return errors.New("window publisher rpc calls: agent is nil")
	}
	zlog.Info("HandleOffer", args.CaptureAudio, args.TargetID)
	publisherSessionID, answer, err := handleOffer(a, args.Offer, args.TargetID, args.CaptureAudio)
	if err != nil {
		return err
	}
	if result != nil {
		result.PublisherSessionID = publisherSessionID
		result.Answer = answer
	}
	return nil
}

func (a *WebRTCPublisher) Close(publisherSessionID string) error {
	if a == nil {
		return errors.New("webRTC publisher rpc calls: agent is nil")
	}
	return a.HandleClose(publisherSessionID)
}

func (s *WebRTCPublishSession) Close() {
	if s.CloseFunc == nil {
		s.CloseFunc()
	}
}
