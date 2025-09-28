package types

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	ID            string                     `json:"id"`
	UserID        string                     `json:"userId,omitempty"`
	Username      string                     `json:"username,omitempty"`
	Conn          *websocket.Conn            `json:"-"`
	PeerConn      *webrtc.PeerConnection     `json:"-"`
	PodcastID     string                     `json:"podcastId,omitempty"`
	RecordingID   string                     `json:"recordingId,omitempty"`
	UserSessionID string                     `json:"userSessionId,omitempty"`
	IsInCall      bool                       `json:"isInCall"`
	RemoteDesc    *webrtc.SessionDescription `json:"-"`
	ConnectedAt   time.Time                  `json:"connectedAt"`
	LastPing      time.Time                  `json:"lastPing"`
	UserAgent     string                     `json:"userAgent,omitempty"`
	IPAddress     string                     `json:"ipAddress,omitempty"`
}

type Podcast struct {
	ID           string             `json:"id"`
	Clients      map[string]*Client `json:"clients"`
	CreatedAt    time.Time          `json:"createdAt"`
	MaxClients   int                `json:"maxClients"`
	HostUserID   string             `json:"hostUserId,omitempty"`
	IsRecording  bool               `json:"isRecording"`
	RecordingID  string             `json:"recordingId,omitempty"`
}

type Message struct {
	Type      string      `json:"type"`
	PodcastID string      `json:"podcastId,omitempty"`
	From      string      `json:"from,omitempty"`
	To        string      `json:"to,omitempty"`
	Payload   interface{} `json:"payload,omitempty"`
	Timestamp int64       `json:"timestamp,omitempty"`
}

type OfferPayload struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

type AnswerPayload struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}

type ICECandidatePayload struct {
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdpMid"`
	SDPMLineIndex int    `json:"sdpMLineIndex"`
}

type RecordingControlPayload struct {
	HostUserID  string `json:"hostUserId"`
	PodcastID   string `json:"podcastId"`
	RecordingID string `json:"recordingId"`
	Timestamp   int64  `json:"timestamp"`
}

type RedisEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	PodcastID string                 `json:"podcastId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

const (
	MessageTypeConnected    = "connected"
	MessageTypeJoinPodcast  = "join-podcast"
	MessageTypeLeavePodcast = "leave-podcast"
	MessageTypePodcastJoined = "podcast-joined"
	MessageTypeUserJoined   = "user-joined"
	MessageTypeUserLeft     = "user-left"
	MessageTypeOffer        = "offer"
	MessageTypeAnswer       = "answer"
	MessageTypeICECandidate = "ice-candidate"
	MessageTypeError        = "error"
	MessageTypeStartRecording = "start-recording"
	MessageTypeStopRecording  = "stop-recording"
	MessageTypeRecordingStarted = "recording-started"
	MessageTypeRecordingStopped = "recording-stopped"
)

const (
	EventClientConnected    = "client_connected"
	EventClientDisconnected = "client_disconnected"
	EventPodcastJoined      = "podcast_joined"
	EventPodcastLeft        = "podcast_left"
	EventCallStarted        = "call_started"
	EventCallEnded          = "call_ended"
	EventRecordingStarted   = "recording_started"
	EventRecordingStopped   = "recording_stopped"
)