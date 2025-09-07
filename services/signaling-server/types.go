package main

import (
	"time"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

type Client struct {
	ID          string                     `json:"id"`
	UserID      string                     `json:"userId,omitempty"`
	Username    string                     `json:"username,omitempty"`
	Conn        *websocket.Conn            `json:"-"`
	PeerConn    *webrtc.PeerConnection     `json:"-"`
	RoomID      string                     `json:"roomId,omitempty"`
	IsInCall    bool                       `json:"isInCall"`
	RemoteDesc  *webrtc.SessionDescription `json:"-"`
	ConnectedAt time.Time                  `json:"connectedAt"`
	LastPing    time.Time                  `json:"lastPing"`
	UserAgent   string                     `json:"userAgent,omitempty"`
	IPAddress   string                     `json:"ipAddress,omitempty"`
}

type Room struct {
	ID         string             `json:"id"`
	Clients    map[string]*Client `json:"clients"`
	CreatedAt  time.Time          `json:"createdAt"`
	MaxClients int                `json:"maxClients"`
}

type Message struct {
	Type      string      `json:"type"`
	RoomID    string      `json:"roomId,omitempty"`
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

type RedisEvent struct {
	EventType string                 `json:"eventType"`
	UserID    string                 `json:"userId,omitempty"`
	ClientID  string                 `json:"clientId"`
	RoomID    string                 `json:"roomId,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp int64                  `json:"timestamp"`
}

const (
	MessageTypeConnected    = "connected"
	MessageTypeJoinRoom     = "join-room"
	MessageTypeLeaveRoom    = "leave-room"
	MessageTypeRoomJoined   = "room-joined"
	MessageTypeUserJoined   = "user-joined"
	MessageTypeUserLeft     = "user-left"
	MessageTypeOffer        = "offer"
	MessageTypeAnswer       = "answer"
	MessageTypeICECandidate = "ice-candidate"
	MessageTypeError        = "error"
)

const (
	EventClientConnected    = "client_connected"
	EventClientDisconnected = "client_disconnected"
	EventRoomJoined         = "room_joined"
	EventRoomLeft           = "room_left"
	EventCallStarted        = "call_started"
	EventCallEnded          = "call_ended"
)
