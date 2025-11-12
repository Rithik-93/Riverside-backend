package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/lakeside/services/signaling-server/types"
)

func (s *SignalingServer) HandleWebSocket(c *gin.Context) {
	accessToken, err := c.Cookie("access_token")
	if err != nil || accessToken == "" {
		log.Printf("WebSocket connection rejected: No access token in cookies")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Access token required"})
		return
	}

	claims, err := s.tokenService.ValidateAccessToken(accessToken)
	if err != nil {
		log.Printf("WebSocket connection rejected: Invalid token - %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	clientID := uuid.New().String()
	client := &types.Client{
		ID:          clientID,
		UserID:      claims.UserID,
		Username:    claims.Username,
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

	log.Printf("Client connected: %s (User: %s)", clientID, claims.Username)

	welcomeMsg := &types.Message{
		Type:      types.MessageTypeConnected,
		Payload:   map[string]string{"clientId": clientID},
		Timestamp: time.Now().Unix(),
	}
	conn.WriteJSON(welcomeMsg)

	defer func() {
		s.removeClient(client)
		log.Printf("Client disconnected: %s", clientID)
	}()

	for {
		var msg types.Message
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

func (s *SignalingServer) handleMessage(client *types.Client, msg *types.Message) {
	switch msg.Type {
	case types.MessageTypeJoinPodcast:
		s.handleJoinPodcast(client, msg)
	case types.MessageTypeLeavePodcast:
		s.handleLeavePodcast(client)
	case types.MessageTypeReady:
		s.handleReady(client, msg)
	case types.MessageTypeOffer:
		s.handleOffer(client, msg)
	case types.MessageTypeAnswer:
		s.handleAnswer(client, msg)
	case types.MessageTypeICECandidate:
		s.handleICECandidate(client, msg)
	case types.MessageTypeStartRecording:
		s.handleStartRecording(client, msg)
	case types.MessageTypeStopRecording:
		s.handleStopRecording(client, msg)
	}
}

func (s *SignalingServer) handleJoinPodcast(client *types.Client, msg *types.Message) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		return
	}

	podcastID, ok := payload["podcastId"].(string)
	if !ok || podcastID == "" {
		if roomID, exists := payload["roomId"].(string); exists && roomID != "" {
			podcastID = roomID
		} else {
			return
		}
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if client.PodcastID == podcastID {
		log.Printf("Client %s is already in podcast %s, ignoring join request", client.ID, podcastID)
		return
	}

	if client.PodcastID != "" {
		s.leavePodcastInternal(client)
	}

	podcast, exists := s.podcasts[podcastID]
	if !exists {
		podcast = &types.Podcast{
			ID:           podcastID,
			Clients:      make(map[string]*types.Client),
			CreatedAt:    time.Now(),
			MaxClients:   10,
			HostUserID:   client.UserID,
			IsRecording:  false,
			ReadyClients: make(map[string]bool),
		}
		s.podcasts[podcastID] = podcast
		
		podcastSessionID := uuid.New().String()
		err := s.redisClient.CreatePodcastSession(podcastSessionID, client.UserID, podcastID)
		if err != nil {
			log.Printf("Failed to create podcast session: %v", err)
		} else {
			log.Printf("Created podcast session: %s for user: %s in podcast: %s", podcastSessionID, client.UserID, podcastID)
		}
	}

	if len(podcast.Clients) >= podcast.MaxClients {
		s.sendError(client.Conn, "Podcast is full")
		return
	}

	podcast.Clients[client.ID] = client
	client.PodcastID = podcastID

	s.redisClient.QueuePodcastJoined(client.ID, client.UserID, podcastID)

	for _, otherClient := range podcast.Clients {
		if otherClient.ID != client.ID {
			joinMsg := &types.Message{
				Type:      types.MessageTypeUserJoined,
				From:      client.ID,
				PodcastID: podcastID,
				Payload: map[string]interface{}{
					"hostUserId":  podcast.HostUserID,
					"isRecording": podcast.IsRecording,
					"recordingId": podcast.RecordingID,
				},
				Timestamp: time.Now().Unix(),
			}
			otherClient.Conn.WriteJSON(joinMsg)
		}
	}

	var users []string
	for id := range podcast.Clients {
		if id != client.ID {
			users = append(users, id)
		}
	}

	podcastMsg := &types.Message{
		Type:      types.MessageTypePodcastJoined,
		PodcastID: podcastID,
		Payload: map[string]interface{}{
			"users":       users,
			"hostUserId":  podcast.HostUserID,
			"isRecording": podcast.IsRecording,
			"recordingId": podcast.RecordingID,
			"podcastId":   podcastID,
		},
		Timestamp: time.Now().Unix(),
	}
	
	log.Printf("Podcast joined - ClientID: %s, HostUserID: %s, IsHost: %v", 
		client.ID, podcast.HostUserID, client.UserID == podcast.HostUserID)
	client.Conn.WriteJSON(podcastMsg)

	log.Printf("Client %s joined podcast %s", client.ID, podcastID)
}

func (s *SignalingServer) handleLeavePodcast(client *types.Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.leavePodcastInternal(client)
}

func (s *SignalingServer) leavePodcastInternal(client *types.Client) {
	if client.PodcastID == "" {
		return
	}

	podcast, exists := s.podcasts[client.PodcastID]
	if !exists {
		client.PodcastID = ""
		return
	}

	podcastID := client.PodcastID
	delete(podcast.Clients, client.ID)
	delete(podcast.ReadyClients, client.ID) // Clear ready state
	client.PodcastID = ""
	client.IsReady = false // Reset ready flag

	s.redisClient.QueuePodcastLeft(client.ID, client.UserID, podcastID)

	for _, otherClient := range podcast.Clients {
		leaveMsg := &types.Message{
			Type:      types.MessageTypeUserLeft,
			From:      client.ID,
			PodcastID: podcastID,
			Payload: map[string]interface{}{
				"hostUserId":  podcast.HostUserID,
				"isRecording": podcast.IsRecording,
				"recordingId": podcast.RecordingID,
			},
			Timestamp: time.Now().Unix(),
		}
		otherClient.Conn.WriteJSON(leaveMsg)
	}

	if len(podcast.Clients) == 0 {
		delete(s.podcasts, podcastID)
	}

	log.Printf("Client %s left podcast %s", client.ID, podcastID)
}

func (s *SignalingServer) handleReady(client *types.Client, msg *types.Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if client.PodcastID == "" {
		log.Printf("Client %s not in a podcast, ignoring ready message", client.ID)
		return
	}

	podcast, exists := s.podcasts[client.PodcastID]
	if !exists {
		log.Printf("Podcast %s not found for ready message", client.PodcastID)
		return
	}

	// Mark this client as ready
	client.IsReady = true
	podcast.ReadyClients[client.ID] = true
	log.Printf("Client %s marked as ready in podcast %s (ready: %d/%d)", 
		client.ID, client.PodcastID, len(podcast.ReadyClients), len(podcast.Clients))

	// Check if exactly 2 clients and both are ready
	if len(podcast.Clients) == 2 && len(podcast.ReadyClients) == 2 {
		log.Printf("Both clients ready in podcast %s, broadcasting both-ready message", client.PodcastID)
		
		// Get sorted client IDs to determine who should initiate
		var clientIDs []string
		for id := range podcast.Clients {
			clientIDs = append(clientIDs, id)
		}
		
		// Sort to determine initiator (lexicographically first)
		var initiatorID, responderID string
		if clientIDs[0] < clientIDs[1] {
			initiatorID = clientIDs[0]
			responderID = clientIDs[1]
		} else {
			initiatorID = clientIDs[1]
			responderID = clientIDs[0]
		}

		// Send both-ready message to all clients with role information
		for id, podcastClient := range podcast.Clients {
			shouldInitiate := (id == initiatorID)
			targetID := responderID
			if id == responderID {
				targetID = initiatorID
			}

			bothReadyMsg := &types.Message{
				Type:      types.MessageTypeBothReady,
				PodcastID: client.PodcastID,
				Payload: map[string]interface{}{
					"shouldInitiate": shouldInitiate,
					"targetUserId":   targetID,
				},
				Timestamp: time.Now().Unix(),
			}
			podcastClient.Conn.WriteJSON(bothReadyMsg)
			log.Printf("Sent both-ready to client %s (shouldInitiate: %v, target: %s)", 
				id, shouldInitiate, targetID)
		}
	}
}

func (s *SignalingServer) handleOffer(client *types.Client, msg *types.Message) {
	s.forwardMessage(client, msg, types.MessageTypeOffer)
}

func (s *SignalingServer) handleAnswer(client *types.Client, msg *types.Message) {
	s.forwardMessage(client, msg, types.MessageTypeAnswer)
}

func (s *SignalingServer) handleICECandidate(client *types.Client, msg *types.Message) {
	s.forwardMessage(client, msg, types.MessageTypeICECandidate)
}

func (s *SignalingServer) forwardMessage(client *types.Client, msg *types.Message, messageType string) {
	if msg.To == "" {
		return
	}

	s.mutex.RLock()
	targetClient, exists := s.clients[msg.To]
	s.mutex.RUnlock()

	if !exists {
		return
	}

	forwardedMsg := &types.Message{
		Type:      messageType,
		From:      client.ID,
		To:        msg.To,
		PodcastID: client.PodcastID,
		Payload:   msg.Payload,
		Timestamp: time.Now().Unix(),
	}

	targetClient.Conn.WriteJSON(forwardedMsg)
}

func (s *SignalingServer) removeClient(client *types.Client) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	delete(s.clients, client.ID)

	if client.PodcastID != "" {
		s.leavePodcastInternal(client)
	}

	if client.UserSessionID != "" {
		err := s.redisClient.DeleteUserSession(client.UserSessionID, client.UserID)
		if err != nil {
			log.Printf("Failed to delete user session: %v", err)
		} else {
			log.Printf("Deleted user session: %s for user: %s", client.UserSessionID, client.UserID)
		}
	}

	if client.PeerConn != nil {
		client.PeerConn.Close()
	}

	duration := time.Since(client.ConnectedAt).Milliseconds()
	s.redisClient.QueueClientDisconnected(client.ID, client.UserID, duration)
}

func (s *SignalingServer) sendError(conn *websocket.Conn, message string) {
	errorMsg := &types.Message{
		Type:      types.MessageTypeError,
		Payload:   map[string]string{"message": message},
		Timestamp: time.Now().Unix(),
	}
	conn.WriteJSON(errorMsg)
}

func (s *SignalingServer) handleStartRecording(client *types.Client, msg *types.Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if client.PodcastID == "" {
		s.sendError(client.Conn, "Not in a podcast")
		return
	}

	podcast, exists := s.podcasts[client.PodcastID]
	if !exists {
		s.sendError(client.Conn, "Podcast not found")
		return
	}

	if podcast.HostUserID != client.UserID {
		s.sendError(client.Conn, "Only the host can control recording")
		return
	}

	if podcast.IsRecording {
		log.Printf("Recording already in progress in podcast %s, ignoring start request", client.PodcastID)
		return
	}

	podcast.IsRecording = true
	recordingID := uuid.New().String()
	podcast.RecordingID = recordingID
	client.RecordingID = recordingID

	var participantUserIDs []string
	for _, podcastClient := range podcast.Clients {
		if podcastClient.UserID != "" {
			participantUserIDs = append(participantUserIDs, podcastClient.UserID)
		}
	}
	
	hostIncluded := false
	for _, userID := range participantUserIDs {
		if userID == client.UserID {
			hostIncluded = true
			break
		}
	}
	if !hostIncluded {
		participantUserIDs = append(participantUserIDs, client.UserID)
	}

	err := s.redisClient.CreateRecordingSessionWithParticipants(recordingID, client.UserID, client.PodcastID, recordingID, participantUserIDs)
	if err != nil {
		log.Printf("Failed to create recording session: %v", err)
	} else {
		log.Printf("Created recording session: %s for user: %s in podcast: %s with %d participants", recordingID, client.UserID, client.PodcastID, len(participantUserIDs))
	}

	recordingMsg := &types.Message{
		Type:      types.MessageTypeRecordingStarted,
		From:      client.ID,
		PodcastID: client.PodcastID,
		Payload: &types.RecordingControlPayload{
			HostUserID:  client.UserID,
			PodcastID:   client.PodcastID,
			RecordingID: recordingID,
			Timestamp:   time.Now().Unix(),
		},
		Timestamp: time.Now().Unix(),
	}

	for _, podcastClient := range podcast.Clients {
		podcastClient.RecordingID = recordingID
		podcastClient.Conn.WriteJSON(recordingMsg)
	}

	s.redisClient.QueueRecordingStarted(client.ID, client.ID, client.PodcastID)

	log.Printf("Recording started by host %s in podcast %s with recording ID %s", client.UserID, client.PodcastID, recordingID)
}

func (s *SignalingServer) handleStopRecording(client *types.Client, msg *types.Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if client.PodcastID == "" {
		s.sendError(client.Conn, "Not in a podcast")
		return
	}

	podcast, exists := s.podcasts[client.PodcastID]
	if !exists {
		s.sendError(client.Conn, "Podcast not found")
		return
	}

	if podcast.HostUserID != client.UserID {
		s.sendError(client.Conn, "Only the host can control recording")
		return
	}

	if !podcast.IsRecording {
		log.Printf("No recording in progress in podcast %s, ignoring stop request", client.PodcastID)
		return
	}

	podcast.IsRecording = false
	recordingID := podcast.RecordingID
	podcast.RecordingID = ""

	if recordingID != "" {
		if err := s.redisClient.EndRecordingSession(recordingID); err != nil {
			log.Printf("Failed to end recording session: %v", err)
		}
	}

	recordingMsg := &types.Message{
		Type:      types.MessageTypeRecordingStopped,
		From:      client.ID,
		PodcastID: client.PodcastID,
		Payload: &types.RecordingControlPayload{
			HostUserID:  client.UserID,
			PodcastID:   client.PodcastID,
			RecordingID: recordingID,
			Timestamp:   time.Now().Unix(),
		},
		Timestamp: time.Now().Unix(),
	}

	for _, podcastClient := range podcast.Clients {
		podcastClient.RecordingID = "" // Clear recording ID
		podcastClient.Conn.WriteJSON(recordingMsg)
	}

	s.redisClient.QueueRecordingStopped(client.ID, client.ID, client.PodcastID)

	log.Printf("Recording stopped by host %s in podcast %s", client.UserID, client.PodcastID)
}

func (s *SignalingServer) GetPodcastInfo(c *gin.Context) {
	podcastID := c.Param("podcastId")

	s.mutex.RLock()
	podcast, exists := s.podcasts[podcastID]
	s.mutex.RUnlock()

	if !exists {
		c.JSON(404, gin.H{"error": "Podcast not found"})
		return
	}

	var clients []string
	for clientID := range podcast.Clients {
		clients = append(clients, clientID)
	}

	c.JSON(200, gin.H{
		"podcastId":   podcastID,
		"clients":     clients,
		"count":       len(clients),
		"isRecording": podcast.IsRecording,
		"recordingId": podcast.RecordingID,
	})
}