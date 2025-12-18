package router

import (
	"fmt"
	"log"

	"game-gateway/internal/backend"
	"game-gateway/internal/session"
	"game-gateway/pkg/protocol"

	"game-protocols/chat"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type SessionManager interface {
	Get(id string) *session.Session
	GetByUserID(userID int32) *session.Session
	Bind(userID int32, sessionID string)
}

type Router struct {
	gameBackends   map[string]*backend.BackendPool
	chatBackends   map[string]*backend.BackendPool
	sessionManager SessionManager
}

func NewRouter() *Router {
	return &Router{
		gameBackends: make(map[string]*backend.BackendPool),
		chatBackends: make(map[string]*backend.BackendPool),
	}
}

func (r *Router) SetBackends(gameBackends map[string]*backend.BackendPool, chatBackends map[string]*backend.BackendPool) {
	r.gameBackends = gameBackends
	r.chatBackends = chatBackends
}

func (r *Router) SetSessionManager(sm SessionManager) {
	r.sessionManager = sm
}

// RoutePacket 使用二进制协议路由数据包
func (r *Router) RoutePacket(s *session.Session, pkt *protocol.Packet) error {
	switch pkt.Route {
	case protocol.RouteChat:
		return r.routeChatPacket(s, pkt)
	case protocol.RouteGame:
		return fmt.Errorf("game route not implemented")
	case protocol.RouteSystem:
		return nil // Heartbeat etc.
	default:
		return fmt.Errorf("unknown route: %d", pkt.Route)
	}
}

// routeChatPacket 处理聊天路由
func (r *Router) routeChatPacket(s *session.Session, pkt *protocol.Packet) error {
	// 解析为 ChatRequest 以获取 GameID
	var req chat.ChatRequest
	if err := proto.Unmarshal(pkt.Payload, &req); err != nil {
		return fmt.Errorf("unmarshal ChatRequest: %w", err)
	}

	if req.Base == nil {
		return fmt.Errorf("missing base info")
	}

	gameID := req.Base.GameId
	if gameID == "" {
		return fmt.Errorf("missing game_id")
	}

	// 自动绑定 UserID（如果还没绑定）
	if s.UserID == 0 && req.Base.UserId > 0 {
		log.Printf("CHAT: Binding Session %s to UserID %d", s.ID, req.Base.UserId)
		r.sessionManager.Bind(req.Base.UserId, s.ID)
		s.UserID = req.Base.UserId
	}

	// 转发到 Chat Service（只发送 Payload，不包含协议头）
	pool, ok := r.chatBackends[gameID]
	if !ok {
		return fmt.Errorf("no chat backend for game: %s", gameID)
	}

	return r.forwardToBackend(pool, s, pkt.Payload)
}

func (r *Router) forwardToBackend(pool *backend.BackendPool, s *session.Session, payload []byte) error {
	conn, err := pool.Get()
	if err != nil {
		return err
	}
	defer pool.Put(conn)

	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (r *Router) HandleBackendMessage(data []byte) {
	// 纯 Protobuf 处理，无需 Envelope
	log.Printf("🔄 [BACKEND-MSG] Received from backend | Size: %d bytes", len(data))

	// 1. 尝试解析 ChatResponse
	var resp chat.ChatResponse
	if err := proto.Unmarshal(data, &resp); err == nil && (resp.TargetUserId > 0 || resp.TargetSessionId != "") {
		log.Printf("📦 [PARSE-RESP] ChatResponse parsed | To: %d, Session: %s, Success: %v",
			resp.TargetUserId, resp.TargetSessionId, resp.Success)

		if err := r.routeToClient(protocol.RouteChat, resp.TargetUserId, resp.TargetSessionId, data); err != nil {
			log.Printf("❌ [ROUTE-FAIL] Failed to route ChatResponse | To: %d, Error: %v", resp.TargetUserId, err)
		} else {
			log.Printf("✅ [ROUTE-OK] ChatResponse routed successfully | To: %d", resp.TargetUserId)
		}
		return
	}

	log.Printf("⚠️  [PARSE-UNKNOWN] Unable to parse message | Size: %d", len(data))
}

func (r *Router) HandleBroadcast(data []byte) {
	// 这是一个来自 MQ 的广播消息
	// 目前我们约定 MQ 中的 "broadcast" topic 只传输 chat.MessageBroadcast

	var broadcast chat.MessageBroadcast
	if err := proto.Unmarshal(data, &broadcast); err != nil {
		log.Printf("❌ [MQ-PARSE-FAIL] Failed to parse broadcast | Error: %v", err)
		return
	}

	if broadcast.TargetUserId == 0 {
		log.Printf("⚠️ [MQ-SKIP] Broadcast with no TargetUserId | Sender: %d", broadcast.SenderId)
		return
	}

	log.Printf("📢 [MQ-RECV] Received broadcast | To: %d, From: %d, Size: %d",
		broadcast.TargetUserId, broadcast.SenderId, len(data))

	if err := r.routeToClient(protocol.RouteChat, broadcast.TargetUserId, "", data); err != nil {
		// target not found 错误在 Gateway 是正常的（如果用户没连这个 Gateway）
		// 但如果是 buffer full 则是问题
		// 降低 "target not found" 的日志级别或忽略，只记录其他错误
		// 这里还是先记录，方便调试
		if err.Error()[:16] != "target not found" {
			log.Printf("❌ [MQ-ROUTE-FAIL] Failed to route broadcast | To: %d, Error: %v", broadcast.TargetUserId, err)
		}
	} else {
		log.Printf("✅ [MQ-ROUTE-OK] Broadcast routed | To: %d", broadcast.TargetUserId)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (r *Router) routeToClient(route protocol.RouteType, userID int32, sessionID string, payload []byte) error {
	var sess *session.Session

	if r.sessionManager == nil {
		return fmt.Errorf("session manager not set")
	}

	// 优先使用 SessionID 路由
	if sessionID != "" {
		sess = r.sessionManager.Get(sessionID)
	}
	// 否则使用 UserID
	if sess == nil && userID > 0 {
		sess = r.sessionManager.GetByUserID(userID)
	}

	if sess == nil {
		return fmt.Errorf("target not found (User: %d, Session: %s)", userID, sessionID)
	}

	// 构建二进制协议包
	pkt := protocol.NewPacket(route, payload)

	// 编码并发送
	encoded := pkt.Encode()

	select {
	case sess.Send <- encoded:
		return nil
	default:
		// 详细的丢弃日志
		bufferUsage := len(sess.Send)
		bufferCap := cap(sess.Send)
		usagePercent := bufferUsage * 100 / bufferCap

		log.Printf("❌ MESSAGE DROPPED - Session buffer full | "+
			"UserID: %d, SessionID: %s, Route: %d, "+
			"BufferUsage: %d/%d (%d%%), PayloadSize: %d bytes",
			userID, sess.ID, route, bufferUsage, bufferCap, usagePercent, len(payload))

		return fmt.Errorf("session %s send buffer full (%d/%d)", sess.ID, bufferUsage, bufferCap)
	}
}
