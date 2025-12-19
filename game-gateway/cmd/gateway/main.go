package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // Import pprof for diagnostic info

	"game-gateway/internal/config"
	"game-gateway/internal/logger"
	"game-gateway/internal/router"
	"game-gateway/internal/server"
	"game-gateway/internal/session"
	"game-pkg/mq"

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

	// 🆕 Enable pprof in non-prod environment
	if cfg.Server.Env != "prod" {
		go func() {
			pprofPort := 6060 // Default pprof port
			log.Printf("📊 Starting pprof server on :%d", pprofPort)
			if err := http.ListenAndServe(fmt.Sprintf(":%d", pprofPort), nil); err != nil {
				log.Printf("⚠️ pprof server failed: %v", err)
			}
		}()
	}

	// 2. Initialize Router first (to handle callbacks)
	r := router.NewRouter()

	// Register backends to Router

	// 4. Initialize Session Manager
	sm := session.NewManager()
	r.SetSessionManager(sm)

	// 5. Initialize MQ
	var mqInstance interface {
		mq.Producer
		mq.Consumer
	}

	switch cfg.MQ.Type {
	case "robustmq":
		log.Println("🚀 Using RobustMQ (MQTT)")
		mqInstance = mq.NewRobustMQ(&mq.RobustMQConfig{
			Broker:   cfg.MQ.RobustMQ.Broker,
			ClientID: cfg.MQ.RobustMQ.ClientID,
			Username: cfg.MQ.RobustMQ.Username,
			Password: cfg.MQ.RobustMQ.Password,
		})
	case "redis":
		fallthrough
	default:
		log.Println("🚀 Using Redis MQ")
		rdb := redis.NewClient(&redis.Options{
			Addr:     cfg.MQ.Redis.Addr,
			Password: cfg.MQ.Redis.Password,
		})
		mqInstance = mq.NewRedisMQ(rdb)
	}

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
