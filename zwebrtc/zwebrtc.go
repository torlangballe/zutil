package zwebrtc

import (
	"github.com/pion/webrtc/v3"
)

type OfferResponse struct {
	SessionID string                    `json:"sessionID"`
	Answer    webrtc.SessionDescription `json:"answer"`
	Error     string                    `json:"error,omitempty"`
}

type CloseRequest struct {
	SessionID string `json:"sessionID"`
}

var DefaultWebRTCConfigurationFunc = func() webrtc.Configuration {
	return webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{
			URLs: []string{"stun:stun.l.google.com:19302"},
		}},
	}
}
