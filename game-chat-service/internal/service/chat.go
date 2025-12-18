package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"game-chat-service/internal/hub"
	"game-chat-service/internal/mq" // New import
	"game-chat-service/internal/repository"
	"game-protocols/chat"

	"google.golang.org/protobuf/proto"
)

type ChatService struct {
	hub      *hub.Hub
	db       *repository.Database
	producer mq.Producer // Changed from GatewaySender
}

func NewChatService(h *hub.Hub, db *repository.Database) *ChatService {
	return &ChatService{
		hub: h,
		db:  db,
	}
}

// SetProducer sets the MQ Producer (e.g. Redis)
func (s *ChatService) SetProducer(p mq.Producer) {
	s.producer = p
}

// HandleRequest processes the incoming chat request from Gateway (or Client via Gateway)
func (s *ChatService) HandleRequest(ctx context.Context, req *chat.ChatRequest) (*chat.ChatResponse, error) {
	startTime := time.Now()
	messageID := fmt.Sprintf("%d->%d:%s", req.Base.UserId, req.ReceiverId, req.Content[:min(20, len(req.Content))])

	log.Printf("📥 [RECV] Message received | From: %d, To: %d, Content: %s, MsgID: %s",
		req.Base.UserId, req.ReceiverId, req.Content[:min(50, len(req.Content))], messageID)

	// 1. Persistence
	dbStart := time.Now()
	msgID, err := s.db.SaveMessage(ctx, req)
	if err != nil {
		log.Printf("❌ [DB-ERROR] Failed to save message | MsgID: %s, Error: %v", messageID, err)
		return nil, fmt.Errorf("persistence failed: %w", err)
	}
	log.Printf("💾 [DB-OK] Message saved | MsgID: %s, DBTime: %v", messageID, time.Since(dbStart))

	// 2. Routing logic (via Hub)
	s.hub.HandleMessage(ctx, req)

	// Response for Sender (ACK)
	resp := &chat.ChatResponse{
		Base:      req.Base,
		Success:   true,
		MessageId: msgID,
		Timestamp: time.Now().Unix(),

		// 🆕 路由信息：发回给发送者
		TargetUserId: req.Base.UserId,
	}

	log.Printf("✅ [ACK-PREPARE] Response ready for sender | To: %d, MsgID: %s", req.Base.UserId, messageID)

	// If private chat, forward to Receiver as well
	// If private chat, forward to Receiver as well
	if req.ReceiverId != 0 && s.producer != nil {
		broadcast := &chat.MessageBroadcast{
			MessageId: msgID,
			SenderId:  req.Base.UserId,
			Content:   req.Content,
			Type:      req.Type,
			Timestamp: req.Base.Timestamp,

			// 🆕 路由信息：告诉 Gateway 发给谁
			TargetUserId: req.ReceiverId,
		}

		log.Printf("📤 [BROADCAST-START] Preparing broadcast | From: %d, To: %d, MsgID: %s",
			req.Base.UserId, req.ReceiverId, messageID)

		// 直接序列化并发送（不包装 Envelope）
		broadcastBytes, err := proto.Marshal(broadcast)
		if err != nil {
			log.Printf("❌ [MARSHAL-ERROR] Failed to marshal broadcast | MsgID: %s, Error: %v", messageID, err)
		} else {
			sendStart := time.Now()
			// 使用 Redis 发布
			if err := s.producer.Publish("broadcast", broadcastBytes); err != nil {
				log.Printf("❌ [SEND-ERROR] Failed to send broadcast | MsgID: %s, Error: %v", messageID, err)
			} else {
				log.Printf("✅ [BROADCAST-OK] Broadcast sent via Redis | To: %d, Size: %d bytes, SendTime: %v, MsgID: %s",
					req.ReceiverId, len(broadcastBytes), time.Since(sendStart), messageID)
			}
		}
	}

	totalTime := time.Since(startTime)
	log.Printf("⏱️  [COMPLETE] Message processing complete | MsgID: %s, TotalTime: %v", messageID, totalTime)

	return resp, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
