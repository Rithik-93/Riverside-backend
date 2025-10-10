# Protobuf - Simple Guide

## For New Team Members (Just Pull & Build!)

```bash
git pull
cd backend/services/upload-service
go build  # Works! No setup needed.
```

**You DON'T need protoc!** Generated `.pb.go` files are already committed.

---

## For Modifying .proto Files

### One-time setup:
```powershell
choco install protoc
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
```

### Regenerate:
```powershell
cd backend/protos
.\generate.ps1
git add .
git commit -m "Update protobuf definitions"
```

---

## What's Inside

- `events.proto` - Redis events
- `upload.proto` - File uploads
- `websocket.proto` - WebSocket messages
- `session.proto` - Session data
- `gen/` - Generated Go code (committed)

---

## Usage Example

```go
import eventspb "github.com/lakeside/backend/protos/gen/events"

// Serialize
event := &eventspb.RedisEvent{EventType: "recording_complete", UserId: "user123"}
data, _ := proto.Marshal(event)

// Deserialize  
newEvent := &eventspb.RedisEvent{}
proto.Unmarshal(data, newEvent)
```

**That's it!** 20-50% smaller than JSON, faster serialization.
