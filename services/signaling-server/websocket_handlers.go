package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

func (s *SignalingServer) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	clientID := uuid.New().String()
	client := &Client{
		ID:          clientID,
		Conn:        conn,
		ConnectedAt: time.Now(),
		LastPing:    time.Now(),
		UserAgent:   c.GetHeader("User-Agent"),
		IPAddress:   c.ClientIP(),
	}

	s.mutex.Lock()
	s.clients[clientID] = client
	s.mutex.Unlock()

	s.redisClient.QueueClientConnected(clientID, client.UserID, client.UserAgent, client.IPAddress)

	log.Printf("Client connected: %s", clientID)

	welcomeMsg := &Message{
		Type:      MessageTypeConnected,
		Payload:   map[string]string{"clientId": clientID},
		Timestamp: time.Now().Unix(),
	}
	conn.WriteJSON(welcomeMsg)

	defer func() {
		s.removeClient(client)
		log.Printf("Client disconnected: %s", clientID)
	}()

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(
				err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived,
			) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		s.handleMessage(client, &msg)
	}
}

func (s *SignalingServer) handleMessage(client *Client, msg *Message) {
	switch msg.Type {
	case MessageTypeJoinRoom:
		s.handleJoinRoom(client, msg)
	case MessageTypeLeaveRoom:
		s.handleLeaveRoom(client)
	case MessageTypeOffer:
		s.handleOffer(client, msg)
	case MessageTypeAnswer:
		s.handleAnswer(client, msg)
	case MessageTypeICECandidate:
		s.handleICECandidate(client, msg)
	}
}

func (s *SignalingServer) handleJoinRoom(client *Client, msg *Message) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	roomID, ok := payload["roomId"].(string)
	if !ok || roomID == "" {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if client.RoomID != "" {
		s.leaveRoomInternal(client)
	}

	room, exists := s.rooms[roomID]
	if !exists {
		room = &Room{
			ID:         roomID,
			Clients:    make(map[string]*Client),
			CreatedAt:  time.Now(),
			MaxClients: 10,
		}
		s.rooms[roomID] = room
	}

	if len(room.Clients) >= room.MaxClients {
		s.sendError(client.Conn, "Room is full")
		return
	}

	room.Clients[client.ID] = client
	client.RoomID = roomID

	s.redisClient.QueueRoomJoined(client.ID, client.UserID, roomID)

	for _, otherClient := range room.Clients {
		if otherClient.ID != client.ID {
			joinMsg := &Message{
				Type:      MessageTypeUserJoined,
				From:      client.ID,
				Timestamp: time.Now().Unix(),
			}
			otherClient.Conn.WriteJSON(joinMsg)
		}
	}

	var users []string
	for id := range room.Clients {
		if id != client.ID {
			users = append(users, id)
		}
	}

	roomMsg := &Message{
		Type:      MessageTypeRoomJoined,
		Payload:   map[string]interface{}{"users": users},
		Timestamp: time.Now().Unix(),
	}
	client.Conn.WriteJSON(roomMsg)

	log.Printf("Client %s joined room %s", client.ID, roomID)
}

func (s *SignalingServer) handleLeaveRoom(client *Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.leaveRoomInternal(client)
}

func (s *SignalingServer) leaveRoomInternal(client *Client) {
	if client.RoomID == "" {
		return
	}

	room, exists := s.rooms[client.RoomID]
	if !exists {
		client.RoomID = ""
		return
	}

	roomID := client.RoomID
	delete(room.Clients, client.ID)
	client.RoomID = ""

	s.redisClient.QueueRoomLeft(client.ID, client.UserID, roomID)

	for _, otherClient := range room.Clients {
		leaveMsg := &Message{
			Type:      MessageTypeUserLeft,
			From:      client.ID,
			Timestamp: time.Now().Unix(),
		}
		otherClient.Conn.WriteJSON(leaveMsg)
	}

	if len(room.Clients) == 0 {
		delete(s.rooms, roomID)
	}

	log.Printf("Client %s left room %s", client.ID, roomID)
}

func (s *SignalingServer) handleOffer(client *Client, msg *Message) {
	s.forwardMessage(client, msg, MessageTypeOffer)
}

func (s *SignalingServer) handleAnswer(client *Client, msg *Message) {
	s.forwardMessage(client, msg, MessageTypeAnswer)
}

func (s *SignalingServer) handleICECandidate(client *Client, msg *Message) {
	s.forwardMessage(client, msg, MessageTypeICECandidate)
}

func (s *SignalingServer) forwardMessage(client *Client, msg *Message, messageType string) {
	if msg.To == "" {
		return
	}

	s.mutex.RLock()
	targetClient, exists := s.clients[msg.To]
	s.mutex.RUnlock()

	if !exists {
		return
	}

	forwardedMsg := &Message{
		Type:      messageType,
		From:      client.ID,
		To:        msg.To,
		Payload:   msg.Payload,
		Timestamp: time.Now().Unix(),
	}

	targetClient.Conn.WriteJSON(forwardedMsg)
}

func (s *SignalingServer) removeClient(client *Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.clients, client.ID)

	if client.RoomID != "" {
		s.leaveRoomInternal(client)
	}

	if client.PeerConn != nil {
		client.PeerConn.Close()
	}

	duration := time.Since(client.ConnectedAt).Milliseconds()
	s.redisClient.QueueClientDisconnected(client.ID, client.UserID, duration)
}

func (s *SignalingServer) sendError(conn *websocket.Conn, message string) {
	errorMsg := &Message{
		Type:      MessageTypeError,
		Payload:   map[string]string{"message": message},
		Timestamp: time.Now().Unix(),
	}
	conn.WriteJSON(errorMsg)
}