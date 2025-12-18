package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"game-chat-service/internal/config"
	"game-chat-service/internal/hub"
	"game-chat-service/internal/logger"
	"game-chat-service/internal/mq"
	"game-chat-service/internal/repository"
	"game-chat-service/internal/service"
	"game-chat-service/internal/transport"

	"game-protocols/chat"
)

type grpcServer struct {
	chat.UnimplementedChatServiceServer
	svc *service.ChatService
}

func main() {
	// Initialize logger first
	logger.Init()
	// Enable debug logging for troubleshooting
	logger.SetLevel(logger.DEBUG)
	logger.EnableTag(logger.TagService)
	logger.EnableTag(logger.TagMQ)
	// Disable noisy logs
	logger.DisableTag(logger.TagDB)
	logger.DisableTag(logger.TagTransport)

	// 1. Load Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}

	// 2. Init DB & Redis
	db, err := repository.NewDatabase(cfg.Database.DSN)
	if err != nil {
		log.Printf("DB Connect error: %v", err)
	}

	rdb, err := repository.NewRedisClient(cfg.Redis.Addr, cfg.Redis.Password)
	if err != nil {
		log.Printf("Redis Connect error: %v", err)
	}

	// 3. Init Core
	h := hub.NewHub(rdb)

	// Initialize RedisMQ
	redisMQ := mq.NewRedisMQ(rdb.Client)

	// Initialize ChatService
	svc := service.NewChatService(h, db)
	svc.SetProducer(redisMQ)

	// 🆕 6. Start Redis Consumer (for Gateway incoming requests)
	requestChan, err := redisMQ.Subscribe("game:request:mmo") // Topic convention
	if err != nil {
		log.Fatalf("Failed to subscribe to requests: %v", err)
	}

	go func() {
		log.Println("🎧 Started listening for Redis requests on game:request:mmo")
		for msg := range requestChan {
			// 并发处理每个请求
			go func(m *mq.Message) {
				var req chat.ChatRequest
				if err := proto.Unmarshal(m.Payload, &req); err != nil {
					log.Printf("Failed to unmarshal request: %v", err)
					return
				}

				// 处理请求
				resp, err := svc.HandleRequest(context.Background(), &req)
				if err != nil {
					log.Printf("HandleRequest error: %v", err)
					// TODO: Send error response?
					return
				}

				// 发送 ACK 响应 (发给发送者)
				if resp != nil {
					// 路由信息
					resp.TargetUserId = req.Base.UserId

					respBytes, err := proto.Marshal(resp)
					if err == nil {
						// 这里的 "broadcast" 其实是 "gateway_downstream" 的意思
						// 所有的 Gateway 都会收到并路由
						if err := redisMQ.Publish("broadcast", respBytes); err != nil {
							log.Printf("Failed to publish ACK: %v", err)
						}
					}
				}
			}(msg)
		}
	}()

	// 4. Start WebSocket Server (for Gateway incoming requests)
	wsSrv := transport.NewWSServer(cfg.Server.Port, svc)

	go func() {
		if err := wsSrv.Start(); err != nil {
			log.Fatalf("WS Server failed: %v", err)
		}
	}()

	// 5. Start gRPC Server
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Server.GrpcPort))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	chat.RegisterChatServiceServer(s, &grpcServer{svc: svc})

	log.Printf("Chat Service listening - WS on :%d, gRPC on :%d", cfg.Server.Port, cfg.Server.GrpcPort)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
