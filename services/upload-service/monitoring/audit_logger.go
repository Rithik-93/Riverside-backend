package monitoring

import (
	"encoding/json"
	"log"
	"os"
	"time"
)

type AuditEvent struct {
	EventType   string                 `json:"event_type"`
	UserID      string                 `json:"user_id"`
	SessionID   string                 `json:"session_id,omitempty"`
	RoomID      string                 `json:"room_id,omitempty"`
	UploadID    string                 `json:"upload_id,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	Details     map[string]interface{} `json:"details"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
}

// AuditLogger handles audit logging
type AuditLogger struct {
	logger *log.Logger
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger() *AuditLogger {
	// Create audit log file
	auditFile, err := os.OpenFile("audit.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Printf("Failed to create audit log file: %v", err)
		return &AuditLogger{
			logger: log.New(os.Stdout, "[AUDIT] ", log.LstdFlags),
		}
	}

	return &AuditLogger{
		logger: log.New(auditFile, "[AUDIT] ", log.LstdFlags),
	}
}

// LogEvent logs an audit event
func (al *AuditLogger) LogEvent(event *AuditEvent) {
	event.Timestamp = time.Now()
	
	eventJSON, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal audit event: %v", err)
		return
	}
	
	al.logger.Println(string(eventJSON))
}

// LogSessionCreated logs session creation
func (al *AuditLogger) LogSessionCreated(userID, sessionID, roomID, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "session_created",
		UserID:    userID,
		SessionID: sessionID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "session_created",
		},
		Success: true,
	})
}

// LogSessionDeleted logs session deletion
func (al *AuditLogger) LogSessionDeleted(userID, sessionID, roomID, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "session_deleted",
		UserID:    userID,
		SessionID: sessionID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "session_deleted",
		},
		Success: true,
	})
}

// LogPresignedURLIssued logs presigned URL issuance
func (al *AuditLogger) LogPresignedURLIssued(userID, roomID, s3Key, fileName, contentType string, fileSize int64, chunkIndex int, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "presigned_url_issued",
		UserID:    userID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action":       "presigned_url_issued",
			"s3_key":       s3Key,
			"file_name":    fileName,
			"content_type": contentType,
			"file_size":    fileSize,
			"chunk_index":  chunkIndex,
		},
		Success: true,
	})
}

// LogPresignedURLFailed logs failed presigned URL issuance
func (al *AuditLogger) LogPresignedURLFailed(userID, roomID, fileName, contentType, errorMsg, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "presigned_url_failed",
		UserID:    userID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action":       "presigned_url_failed",
			"file_name":    fileName,
			"content_type": contentType,
		},
		Success: false,
		Error:   errorMsg,
	})
}

// LogChunkUploaded logs chunk upload
func (al *AuditLogger) LogChunkUploaded(userID, roomID, s3Key, fileName string, fileSize int64, chunkIndex int, isFinal bool) {
	al.LogEvent(&AuditEvent{
		EventType: "chunk_uploaded",
		UserID:    userID,
		RoomID:    roomID,
		Details: map[string]interface{}{
			"action":       "chunk_uploaded",
			"s3_key":       s3Key,
			"file_name":    fileName,
			"file_size":    fileSize,
			"chunk_index":  chunkIndex,
			"is_final":     isFinal,
		},
		Success: true,
	})
}

// LogUploadSessionStarted logs upload session start
func (al *AuditLogger) LogUploadSessionStarted(userID, roomID, uploadID, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "upload_session_started",
		UserID:    userID,
		RoomID:    roomID,
		UploadID:  uploadID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "upload_session_started",
		},
		Success: true,
	})
}

// LogUploadSessionFinalized logs upload session finalization
func (al *AuditLogger) LogUploadSessionFinalized(userID, roomID, uploadID string, chunkCount int, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "upload_session_finalized",
		UserID:    userID,
		RoomID:    roomID,
		UploadID:  uploadID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action":       "upload_session_finalized",
			"chunk_count":  chunkCount,
		},
		Success: true,
	})
}

// LogUploadSessionRevoked logs upload session revocation
func (al *AuditLogger) LogUploadSessionRevoked(userID, roomID, uploadID, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "upload_session_revoked",
		UserID:    userID,
		RoomID:    roomID,
		UploadID:  uploadID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "upload_session_revoked",
		},
		Success: true,
	})
}

// LogSessionValidationFailed logs failed session validation
func (al *AuditLogger) LogSessionValidationFailed(userID, roomID, reason, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: "session_validation_failed",
		UserID:    userID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "session_validation_failed",
			"reason": reason,
		},
		Success: false,
		Error:   reason,
	})
}

// LogSecurityEvent logs security-related events
func (al *AuditLogger) LogSecurityEvent(eventType, userID, roomID, details, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: eventType,
		UserID:    userID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action":  "security_event",
			"details": details,
		},
		Success: false,
		Error:   details,
	})
}

// LogError logs general errors
func (al *AuditLogger) LogError(eventType, userID, roomID, errorMsg, ipAddress, userAgent string) {
	al.LogEvent(&AuditEvent{
		EventType: eventType,
		UserID:    userID,
		RoomID:    roomID,
		IPAddress: ipAddress,
		UserAgent: userAgent,
		Details: map[string]interface{}{
			"action": "error",
		},
		Success: false,
		Error:   errorMsg,
	})
}

// Global audit logger instance
var auditLogger *AuditLogger

// Exported AuditLogger for direct access
var Logger *AuditLogger

// InitializeAuditLogger initializes the global audit logger
func InitializeAuditLogger() {
	auditLogger = NewAuditLogger()
	Logger = auditLogger
	log.Println("Audit logger initialized")
}

