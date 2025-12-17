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
	gameBackends map[string]*backend.BackendPool
	chatBackends map[string]*backend.BackendPool
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

func (r *Router) RouteMessage(s *session.Session, data []byte) error {
	return fmt.Errorf("deprecated: use binary protocol")
}

func (r *Router) forwardToBackend(pool *backend.BackendPool, s *session.Session, payload []byte, additionalHeader []byte) error {
	conn, err := pool.Get()
	if err != nil {
		return err
	}
	defer pool.Put(conn)

	return conn.WriteMessage(websocket.BinaryMessage, payload)
}

func (r *Router) HandleBackendMessage(data []byte) {
    // 纯 Protobuf 处理，无需 Envelope
    // log.Printf("HandleBackendMessage: Received %d bytes", len(data))

    // 1. 尝试解析 ChatResponse
    var resp chat.ChatResponse
    if err := proto.Unmarshal(data, &resp); err == nil && (resp.TargetUserId > 0 || resp.TargetSessionId != "") {
        if err := r.routeToClient(protocol.RouteChat, resp.TargetUserId, resp.TargetSessionId, data); err != nil {
            log.Printf("Failed to route ChatResponse: %v", err)
        }
        return
    }

    // 2. 尝试解析 MessageBroadcast
    var broadcast chat.MessageBroadcast
    if err := proto.Unmarshal(data, &broadcast); err == nil && broadcast.TargetUserId > 0 {
        if err := r.routeToClient(protocol.RouteChat, broadcast.TargetUserId, "", data); err != nil {
             log.Printf("Failed to route MessageBroadcast: %v", err)
        }
        return
    }
    
    log.Printf("HandleBackendMessage: Unable to route message (size=%d)", len(data))
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
        return fmt.Errorf("session %s send buffer full", sess.ID)
    }
}
