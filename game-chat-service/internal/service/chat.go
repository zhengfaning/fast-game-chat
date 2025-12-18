package service

import (
	"context"
	"fmt"
	"time"

	"game-chat-service/internal/hub"
	"game-chat-service/internal/logger"
	"game-chat-service/internal/mq" // New import
	"game-chat-service/internal/repository"
	"game-protocols/chat"

	"google.golang.org/protobuf/proto"
)

type ChatService struct {
	hub      *hub.Hub
	db       *repository.Database
	producer mq.Producer // Changed from GatewaySender
	saveChan chan *chat.ChatRequest
}

func NewChatService(h *hub.Hub, db *repository.Database) *ChatService {
	s := &ChatService{
		hub:      h,
		db:       db,
		saveChan: make(chan *chat.ChatRequest, 20000), // Large buffer to absorb bursts
	}

	// Start DB workers
	// 50 workers to handle DB writes concurrently
	for i := 0; i < 50; i++ {
		go s.dbWorker()
	}

	return s
}

// dbWorker consumes requests from channel and persists them
func (s *ChatService) dbWorker() {
	for req := range s.saveChan {
		_, err := s.db.SaveMessage(context.Background(), req)
		if err != nil {
			logger.Error(logger.TagDB, "Async save failed | User: %d, Error: %v", req.Base.UserId, err)
		}
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

	logger.Debug(logger.TagService, "Message received | From: %d, To: %d, Content: %s, MsgID: %s",
		req.Base.UserId, req.ReceiverId, req.Content[:min(50, len(req.Content))], messageID)

	// 1. Persistence (Async)
	// Push to channel, non-blocking if buffer has space
	var msgID int64
	select {
	case s.saveChan <- req:
		msgID = time.Now().UnixNano() // Temporary int64 ID
	default:
		logger.Error(logger.TagService, "Save buffer full | Dropping message from %d", req.Base.UserId)
		return nil, fmt.Errorf("server overload (db buffer full)")
	}

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

	logger.Debug(logger.TagService, "Response ready for sender | To: %d, MsgID: %s", req.Base.UserId, messageID)

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

		logger.Debug(logger.TagMQ, "Preparing broadcast | From: %d, To: %d, MsgID: %s",
			req.Base.UserId, req.ReceiverId, messageID)

		// 直接序列化并发送（不包装 Envelope）
		broadcastBytes, err := proto.Marshal(broadcast)
		if err != nil {
			logger.Error(logger.TagMQ, "Failed to marshal broadcast | MsgID: %s, Error: %v", messageID, err)
		} else {
			sendStart := time.Now()
			// 使用 Redis 发布
			if err := s.producer.Publish("broadcast", broadcastBytes); err != nil {
				logger.Error(logger.TagMQ, "Failed to send broadcast | MsgID: %s, Error: %v", messageID, err)
			} else {
				logger.Debug(logger.TagMQ, "Broadcast sent via Redis | To: %d, Size: %d bytes, SendTime: %v, MsgID: %s",
					req.ReceiverId, len(broadcastBytes), time.Since(sendStart), messageID)
			}
		}
	}

	totalTime := time.Since(startTime)
	logger.Debug(logger.TagPerf, "Message processing complete | MsgID: %s, TotalTime: %v", messageID, totalTime)

	return resp, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
