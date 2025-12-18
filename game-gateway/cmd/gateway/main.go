package main

import (
	"fmt"
	"log"

	"game-gateway/internal/backend"
	"game-gateway/internal/config"
	"game-gateway/internal/logger"
	"game-gateway/internal/mq"
	"game-gateway/internal/router"
	"game-gateway/internal/server"
	"game-gateway/internal/session"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Initialize logger first
	logger.Init()
	// Enable debug logging for troubleshooting
	logger.SetLevel(logger.DEBUG)
	logger.EnableTag(logger.TagRouter)
	logger.EnableTag(logger.TagMQ)
	// Disable noisy logs
	logger.DisableTag(logger.TagSession)
	logger.DisableTag(logger.TagProtocol)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Router first (to handle callbacks)
	r := router.NewRouter()

	// Define backend message handler
	handler := func(data []byte) {
		r.HandleBackendMessage(data)
	}

	// 3. Initialize Backend Pools
	gameBackends := make(map[string]*backend.BackendPool)
	chatBackends := make(map[string]*backend.BackendPool)

	for _, game := range cfg.Games {
		log.Printf("Initializing backends for game: %s", game.ID)

		gameBackends[game.ID] = backend.NewBackendPool(
			game.GameBackend.Host,
			game.GameBackend.Port,
			game.GameBackend.PoolSize,
			handler,
		)

		chatBackends[game.ID] = backend.NewBackendPool(
			game.ChatBackend.Host,
			game.ChatBackend.Port,
			game.ChatBackend.PoolSize,
			handler,
		)
	}

	// Register backends to Router
	r.SetBackends(gameBackends, chatBackends)

	// 4. Initialize Session Manager
	sm := session.NewManager()
	r.SetSessionManager(sm)

	// 5. Initialize MQ Consumer (Redis)
	// We need to create a redis client here as config has redis info
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
	})

	// Create MQ instance
	mqInstance := mq.NewRedisMQ(rdb)

	// Subscribe to broadcasts
	msgChan, err := mqInstance.Subscribe("broadcast")
	if err != nil {
		log.Fatalf("Failed to subscribe to broadcast: %v", err)
	}

	// Start consumer loop
	go func() {
		log.Println("🎧 Started listening for Redis broadcasts")
		for msg := range msgChan {
			r.HandleBroadcast(msg.Payload)
		}
	}()

	// 6. Start Server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	srv := server.NewServer(addr, r, sm)

	if err := srv.Start(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
