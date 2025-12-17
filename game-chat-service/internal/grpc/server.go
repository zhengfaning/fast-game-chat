package grpc

import (
    "context"

    "game-chat-service/internal/service"
    "game-protocols/chat"
)

type Server struct {
    chat.UnimplementedChatServiceServer
    svc *service.ChatService
}

func NewServer(svc *service.ChatService) *Server {
    return &Server{svc: svc}
}

func (s *Server) ValidateAuthToken(ctx context.Context, req *chat.AuthTokenRequest) (*chat.UserIdentity, error) {
    // Stage 2: Mock implementation
    // In real world, this calls GLS or checks Redis/Token Service
    
    // Mock: Token "123" -> User 1001, Game "mmo"
    if req.Token == "123" {
        return &chat.UserIdentity{
            UserId: 1001,
            GameId: "mmo",
            Valid:  true,
        }, nil
    }
    
    return &chat.UserIdentity{Valid: false}, nil
}

func (s *Server) SendSystemBroadcast(ctx context.Context, req *chat.SystemBroadcastRequest) (*chat.Empty, error) {
    // Logic to broadcast via Hub/Redis
    return &chat.Empty{}, nil
}

func (s *Server) KickUser(ctx context.Context, req *chat.KickUserRequest) (*chat.Empty, error) {
    // Logic to kick user
    return &chat.Empty{}, nil
}
