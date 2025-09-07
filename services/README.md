# WebRTC Video Calling System

This is a complete WebRTC video calling system built with Pion WebRTC for the backend and React for the frontend.

## Architecture

- **Signaling Server** (`signaling-server/`): Basic WebSocket-based signaling server for coordinating WebRTC connections
- **WebRTC Service** (`webrtc-service/`): Advanced Pion WebRTC service with media forwarding capabilities
- **Frontend Client** (`../../client/`): React-based video calling interface

## Features

- ✅ Real-time video and audio calling
- ✅ Room-based communication
- ✅ Multiple users per room
- ✅ Modern, responsive UI
- ✅ ICE candidate exchange
- ✅ Media track forwarding
- ✅ Connection state management
- ✅ Mobile-friendly design

## Quick Start

### 1. Start the Signaling Server

For basic signaling (recommended for development):

```bash
cd Lakeside/services/signaling-server
go mod tidy
go run main.go
```

The server will start on `http://localhost:8080`

### 2. Start the Frontend

```bash
cd client
npm install
npm run dev
```

The frontend will be available at `http://localhost:5173`

### 3. Test the Video Calling

1. Open two browser windows/tabs to `http://localhost:5173`
2. Enter the same room ID in both windows (e.g., "test-room")
3. Click "Join Room" in both windows
4. Click the "📞 Call" button next to the other user
5. Allow camera/microphone permissions when prompted
6. Enjoy your video call!

## Advanced Setup (Optional)

### Using the Advanced WebRTC Service

For production or advanced features, use the Pion WebRTC service instead:

```bash
cd Lakeside/services/webrtc-service
go mod tidy
go run main.go
```

This service includes:
- Media track forwarding
- Advanced codec support (VP8, H.264, Opus)
- NACK and PLI support for better video quality
- Comprehensive error handling
- Statistics and reporting

## Configuration

### STUN/TURN Servers

The default configuration uses Google's STUN servers:
- `stun:stun.l.google.com:19302`
- `stun:stun1.l.google.com:19302`

For production, consider using your own TURN servers for better NAT traversal.

### Supported Codecs

- **Video**: VP8, H.264
- **Audio**: Opus

## Troubleshooting

### Common Issues

1. **Camera/Microphone Access Denied**
   - Ensure you're using HTTPS or localhost
   - Check browser permissions
   - Try a different browser

2. **Connection Failed**
   - Check firewall settings
   - Ensure STUN servers are accessible
   - Consider using TURN servers for restrictive networks

3. **No Video/Audio**
   - Check browser developer console for errors
   - Verify media devices are working
   - Check codec compatibility

### Browser Support

- Chrome 80+
- Firefox 75+
- Safari 14+
- Edge 80+

## Development

### Project Structure

```
Lakeside/services/
├── signaling-server/          # Basic WebSocket signaling
│   ├── main.go
│   └── go.mod
├── webrtc-service/           # Advanced Pion WebRTC service
│   ├── main.go
│   └── go.mod
└── README.md

client/
├── src/
│   ├── App.tsx              # Main React component
│   ├── App.css              # Styles
│   └── main.tsx
├── package.json
└── vite.config.ts
```

### Adding Features

1. **Screen Sharing**: Modify `getUserMedia` calls to include screen capture
2. **Chat Messages**: Extend WebSocket message types
3. **Recording**: Add media recording capabilities
4. **Multiple Rooms**: Enhance room management
5. **User Authentication**: Integrate with auth services

## Security Considerations

- Use HTTPS in production
- Implement proper authentication
- Validate all WebSocket messages
- Rate limit connections
- Use secure TURN servers

## Performance Tips

- Optimize video resolution based on connection quality
- Implement adaptive bitrate streaming
- Use efficient codecs (VP9, AV1 for modern browsers)
- Monitor connection statistics
- Implement graceful degradation

## License

This project is part of the Lakeside microservices architecture.
