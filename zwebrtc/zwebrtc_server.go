//go:build server

package zwebrtc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/pion/webrtc/v3"
	"github.com/torlangballe/zutil/zlog"
	"github.com/torlangballe/zutil/zrest"
)

type PublishSession interface {
	Close()
}

type PublisherOfferFunc func(req OfferRequest) (publisherSessionID string, answer webrtc.SessionDescription, err error)
type PublisherCloseFunc func(targetID, publisherSessionID string) error

type WebRTCServerConfig struct {
	WebRTCConfig       webrtc.Configuration
	SingleSession      bool
	OfferTimeout       time.Duration
	PublisherOfferFunc PublisherOfferFunc
	PublisherCloseFunc PublisherCloseFunc
}

type OfferRequest struct {
	Offer        webrtc.SessionDescription `json:"offer"`
	TargetID     string                    `json:"target,omitempty"`
	CaptureAudio bool                      `json:"captureAudio,omitempty"`
}

// type CloseResponse struct {
// 	Closed bool   `json:"closed"`
// 	Error  string `json:"error,omitempty"`
// }

type ConfigResponse struct {
	DefaultTargetID string `json:"defaultTarget,omitempty"`
}

type WebRTCServer struct {
	cfg WebRTCServerConfig

	mu       sync.Mutex
	sessions map[string]PublishSession
	nextID   uint64
}

type directPublishSession struct {
	closeFunc func()
}

func NewWebRTCServer(router *mux.Router, cfg WebRTCServerConfig) *WebRTCServer {
	s := &WebRTCServer{
		cfg:      cfg,
		sessions: map[string]PublishSession{},
		nextID:   0,
	}
	if s.cfg.OfferTimeout <= 0 {
		s.cfg.OfferTimeout = 25 * time.Second
	}
	zlog.Info("NewWebRTCServer: addOffer")
	zrest.AddHandler(router, "webrtc/offer", s.HandleOffer).Methods(http.MethodPost)
	// zrest.AddHandler(router, "webrtc/close", s.HandleClose).Methods(http.MethodPost)
	zrest.AddHandler(router, "webrtc/config", s.HandleConfig).Methods(http.MethodGet)
	zrest.AddHandler(router, "healthz", s.HandleHealth)
	return s
}

func (s *WebRTCServer) SessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func (s *WebRTCServer) CloseAll() {
	s.mu.Lock()
	sessions := s.sessions
	s.sessions = map[string]PublishSession{}
	s.mu.Unlock()
	for _, sess := range sessions {
		sess.Close()
	}
}

func (s *WebRTCServer) newSessionID() string {
	n := atomic.AddUint64(&s.nextID, 1)
	return fmt.Sprintf("s-%d", n)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *WebRTCServer) HandleOffer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, OfferResponse{Error: "method must be POST"})
		return
	}
	defer r.Body.Close()

	var req OfferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, OfferResponse{Error: "invalid JSON body"})
		return
	}
	if req.Offer.Type != webrtc.SDPTypeOffer || req.Offer.SDP == "" {
		writeJSON(w, http.StatusBadRequest, OfferResponse{Error: "offer must contain type=offer and non-empty sdp"})
		return
	}
	if s.cfg.SingleSession {
		s.CloseAll()
	}
	if s.cfg.PublisherOfferFunc == nil {
		writeJSON(w, http.StatusBadGateway, OfferResponse{Error: "publisher signaling is not configured"})
		return
	}

	type offerResult struct {
		publisherSessionID string
		answer             webrtc.SessionDescription
		err                error
	}
	resultCh := make(chan offerResult, 1)
	go func() {
		publisherSessionID, answer, err := s.cfg.PublisherOfferFunc(req)
		resultCh <- offerResult{publisherSessionID: publisherSessionID, answer: answer, err: err}
	}()

	var result offerResult
	select {
	case result = <-resultCh:
	case <-time.After(s.cfg.OfferTimeout):
		writeJSON(w, http.StatusGatewayTimeout, OfferResponse{Error: "timed out waiting for publisher"})
		return
	}

	if result.err != nil {
		writeJSON(w, http.StatusBadGateway, OfferResponse{Error: result.err.Error()})
		return
	}
	if result.publisherSessionID == "" {
		writeJSON(w, http.StatusBadGateway, OfferResponse{Error: "publisher did not return session id"})
		return
	}

	sid := s.newSessionID()
	session := &directPublishSession{closeFunc: func() {
		if s.cfg.PublisherCloseFunc != nil {
			_ = s.cfg.PublisherCloseFunc(req.TargetID, result.publisherSessionID)
		}
	}}
	s.mu.Lock()
	s.sessions[sid] = session
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, OfferResponse{SessionID: sid, Answer: result.answer})
}

func (s *directPublishSession) Close() {
	if s == nil || s.closeFunc == nil {
		return
	}
	s.closeFunc()
}

func (s *WebRTCServer) Close(sessionID string) error {
	zlog.Info("WebRTCServer.Close:", sessionID)
	s.mu.Lock()
	sess, ok := s.sessions[sessionID]
	if ok {
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()
	if !ok {
		return zlog.Error("session not found", sessionID)
	}
	sess.Close()
	return nil
}

func (s *WebRTCServer) HandleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sessions": s.SessionCount()})
}

func (s *WebRTCServer) HandleConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, ConfigResponse{})
}
