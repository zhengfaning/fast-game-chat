package service

import (
    "context"
    "fmt"
    "log"
    "time"
    
    "game-chat-service/internal/hub"
    "game-chat-service/internal/repository"
    "game-protocols/chat"
    "google.golang.org/protobuf/proto"
)

type GatewaySender interface {
    // 发送纯 Protobuf 数据到 Gateway
    // Gateway 会根据消息中的路由字段（TargetUserId）进行路由
    SendToGateway(data []byte) error
}

type ChatService struct {
    hub  *hub.Hub
    db   *repository.Database
    sender GatewaySender // Added sender field
}

func NewChatService(h *hub.Hub, db *repository.Database) *ChatService {
    return &ChatService{
        hub: h,
        db:  db,
    }
}

// SetSender sets the GatewaySender for the ChatService.
func (s *ChatService) SetSender(sender GatewaySender) {
    s.sender = sender
}

// HandleRequest processes the incoming chat request from Gateway (or Client via Gateway)
func (s *ChatService) HandleRequest(ctx context.Context, req *chat.ChatRequest) (*chat.ChatResponse, error) {
    log.Printf("Processing message from user %d to %d", req.Base.UserId, req.ReceiverId)
    
    // 1. Persistence
    msgID, err := s.db.SaveMessage(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("persistence failed: %w", err)
    }
    
    // 2. Routing logic (via Hub)
    // For Stage 2, we just log.
    s.hub.HandleMessage(ctx, req)
    
    // Response for Sender (ACK)
    resp := &chat.ChatResponse{
        Base: req.Base,
        Success: true,
        MessageId: msgID,
        Timestamp: time.Now().Unix(),
        
        // 🆕 路由信息：发回给发送者
        TargetUserId: req.Base.UserId,
    }
    
    // If private chat, forward to Receiver as well
    if req.ReceiverId != 0 && s.sender != nil {
        // 🎯 新架构：直接发送纯 Protobuf，不使用 Envelope
        // Gateway 会读取 MessageBroadcast.TargetUserId 来路由
        
        broadcast := &chat.MessageBroadcast{
             MessageId: msgID,
             SenderId: req.Base.UserId,
             Content: req.Content,
             Type: req.Type,
             Timestamp: req.Base.Timestamp,
             
             // 🆕 路由信息：告诉 Gateway 发给谁
             TargetUserId: req.ReceiverId,
        }
        
        log.Printf("Sending broadcast to User %d: SenderId=%d, Content=%s", req.ReceiverId, req.Base.UserId, req.Content)
        
        // 直接序列化并发送（不包装 Envelope）
        broadcastBytes, err := proto.Marshal(broadcast)
        if err != nil {
            log.Printf("Failed to marshal broadcast: %v", err)
        } else if err := s.sender.SendToGateway(broadcastBytes); err != nil {
            log.Printf("Failed to send broadcast to gateway: %v", err)
        } else {
            log.Printf("Broadcast sent successfully to gateway (%d bytes)", len(broadcastBytes))
        }
    }
    
    return resp, nil
}
