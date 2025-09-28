package handlers

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lakeside/services/signaling-server/config"
	"github.com/lakeside/services/signaling-server/internal"
	"github.com/lakeside/services/signaling-server/types"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type SignalingServer struct {
	podcasts    map[string]*types.Podcast
	clients     map[string]*types.Client
	redisClient *config.RedisClient
	tokenService *internal.TokenService
	mutex       sync.RWMutex
}

func NewSignalingServer(redisClient *config.RedisClient, tokenService *internal.TokenService) *SignalingServer {
	return &SignalingServer{
		podcasts:    make(map[string]*types.Podcast),
		clients:     make(map[string]*types.Client),
		redisClient: redisClient,
		tokenService: tokenService,
	}
}