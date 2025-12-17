package router

import (
    "fmt"
    "log"

    "game-gateway/pkg/protocol"
    "game-gateway/internal/session"
    "game-protocols/chat"
    "google.golang.org/protobuf/proto"
)

// RoutePacket 使用二进制协议路由数据包
func (r *Router) RoutePacket(s *session.Session, pkt *protocol.Packet) error {
    switch pkt.Route {
    case protocol.RouteChat:
        return r.routeChatPacket(s, pkt)
    case protocol.RouteGame:
        return fmt.Errorf("game route not implemented/verified yet")
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
    
    // Use existing forward logic
    return r.forwardToBackend(pool, s, pkt.Payload, nil)
}
