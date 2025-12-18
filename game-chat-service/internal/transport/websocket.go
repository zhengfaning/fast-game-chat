package transport

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"

	"game-chat-service/internal/service"
	"game-protocols/chat"
)

type WSServer struct {
	addr     string
	svc      *service.ChatService
	upgrader websocket.Upgrader

	// Active key-value pairs? No on-connection context needed globally.
}

func NewWSServer(port int, svc *service.ChatService) *WSServer {
	return &WSServer{
		addr: fmt.Sprintf(":%d", port),
		svc:  svc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  8192, // 增加到 8KB
			WriteBufferSize: 8192, // 增加到 8KB
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

func (s *WSServer) Start() error {
	http.HandleFunc("/", s.handleConnection) // Gateway connects to root
	log.Printf("Chat Service (WS) listening on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *WSServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 创建写入队列（增大缓冲区以支持高并发）
	writeChan := make(chan []byte, 512) // 增加到 512

	defer func() {
		close(writeChan)
		conn.Close()
	}()

	// 启动写入 pump，专门负责写入操作
	done := make(chan struct{})
	go func() {
		defer close(done)
		for data := range writeChan {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Printf("Write error: %v", err)
				return
			}
			log.Printf("WritePump: Successfully wrote %d bytes to Gateway", len(data))
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WS error: %v", err)
			}
			break
		}

		// 🎯 新架构：直接解析 ChatRequest (纯 Protobuf)
		// Gateway 已经提取了 Payload 并转发给我们
		var req chat.ChatRequest
		if err := proto.Unmarshal(message, &req); err != nil {
			log.Printf("Failed to unmarshal ChatRequest: %v", err)
			continue
		}

		log.Printf("Received ChatRequest from User %d to User %d: %s",
			req.Base.UserId, req.ReceiverId, req.Content)

		// Handle Request
		resp, err := s.svc.HandleRequest(context.Background(), &req)
		if err != nil {
			log.Printf("Handle error: %v", err)
			continue
		}

		// Send Response (纯 Protobuf ChatResponse)
		if resp != nil {
			respBytes, err := proto.Marshal(resp)
			if err != nil {
				log.Printf("Marshal response error: %v", err)
				continue
			}

			// 通过写入队列发送（不直接写入）
			select {
			case writeChan <- respBytes:
				log.Printf("Queued ChatResponse to Gateway (%d bytes)", len(respBytes))
			default:
				// 详细的丢弃日志
				bufferUsage := len(writeChan)
				bufferCap := cap(writeChan)
				usagePercent := bufferUsage * 100 / bufferCap

				log.Printf("❌ RESPONSE DROPPED - Write channel full | "+
					"BufferUsage: %d/%d (%d%%), ResponseSize: %d bytes, "+
					"FromUser: %d, ToUser: %d",
					bufferUsage, bufferCap, usagePercent, len(respBytes),
					req.Base.UserId, resp.TargetUserId)
			}
		}
	}

	// 等待写入完成
	<-done
}
