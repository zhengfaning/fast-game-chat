package main

import (
    "fmt"
    "log"
    "net"

    "game-chat-service/internal/config"
    "game-chat-service/internal/hub"
    "game-chat-service/internal/repository"
    "game-chat-service/internal/service"
    "game-chat-service/internal/transport"

    "game-protocols/chat"
    "google.golang.org/grpc"
)

type grpcServer struct {
    chat.UnimplementedChatServiceServer
    svc *service.ChatService
}

func main() {
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
    svc := service.NewChatService(h, db)

    // 4. Start WebSocket Server (for Gateway)
    // Start in a goroutine so it doesn't block gRPC startup (or vice versa)
    wsSrv := transport.NewWSServer(cfg.Server.Port, svc)
    svc.SetSender(wsSrv) // Wiring: Service uses WS Server to push messages
    
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
