package transport

import (
	"log"
	"net/http"
    "fmt"
    "context"
    "sync"

	"github.com/gorilla/websocket"
    "google.golang.org/protobuf/proto"
    
    "game-chat-service/internal/service"
    "game-protocols/chat"
)

type WSServer struct {
	addr    string
    svc     *service.ChatService
	upgrader websocket.Upgrader
    
    // Active connection to Gateway (Simplified: assumption 1 active gateway conn)
    activeConn *websocket.Conn
    mu         sync.Mutex
}

func NewWSServer(port int, svc *service.ChatService) *WSServer {
	return &WSServer{
		addr: fmt.Sprintf(":%d", port),
        svc:  svc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
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

// SendToGateway 发送纯 Protobuf 数据到 Gateway
// 不再使用 Envelope 包装
func (s *WSServer) SendToGateway(data []byte) error {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if s.activeConn == nil {
        return fmt.Errorf("no active gateway connection")
    }
    
    log.Printf("WSServer.SendToGateway: Sending %d bytes to Gateway", len(data))
    
    // 直接发送 Protobuf 数据
    return s.activeConn.WriteMessage(websocket.BinaryMessage, data)
}

func (s *WSServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
    
    s.mu.Lock()
    s.activeConn = conn
    s.mu.Unlock()
    
    defer func() {
        s.mu.Lock()
        if s.activeConn == conn {
            s.activeConn = nil
        }
        s.mu.Unlock()
        conn.Close()
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
            
            // 直接发送 Protobuf（不包装 Envelope）
            // Gateway 会读取 resp.TargetUserId 来路由
            if err := conn.WriteMessage(websocket.BinaryMessage, respBytes); err != nil {
                log.Println("Write error:", err)
                break
            }
            
            log.Printf("Sent ChatResponse to Gateway (%d bytes)", len(respBytes))
        }
	}
}
