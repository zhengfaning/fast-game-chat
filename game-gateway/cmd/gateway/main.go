package main

import (
	"fmt"
	"log"

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
	logger.EnableTag(logger.TagProtocol)
	// Disable noisy logs
	logger.DisableTag(logger.TagSession)

	// 1. Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Router first (to handle callbacks)
	r := router.NewRouter()



	// Register backends to Router

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

	// Inject MQ into Router to enable async request processing
	r.SetMQ(mqInstance)

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
