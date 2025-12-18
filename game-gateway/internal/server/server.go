package server

import (
	"log"
	"net/http"
	"time"

	"game-gateway/internal/router"
	"game-gateway/internal/session"
	"game-gateway/pkg/protocol"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Server struct {
	addr     string
	router   *router.Router
	sessions *session.Manager
	upgrader websocket.Upgrader
}

func NewServer(addr string, r *router.Router, s *session.Manager) *Server {
	return &Server{
		addr:     addr,
		router:   r,
		sessions: s,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  8192, // 增加到 8KB
			WriteBufferSize: 8192, // 增加到 8KB
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev
			},
		},
	}
}

func (s *Server) Start() error {
	http.HandleFunc("/ws", s.handleConnection)
	log.Printf("Gateway listening on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	// 创建协议包装的 WebSocket 连接
	wsConn := protocol.NewWSConn(conn)
	wsConn.SetReadLimit(16 * 1024 * 1024) // 16MB

	// Create session with larger buffer for high concurrency
	sess := &session.Session{
		ID:        uuid.New().String(),
		Conn:      conn,                    // 保留原始连接用于底层操作
		Send:      make(chan []byte, 1024), // 增加到 1024
		AuthToken: "",
	}
	s.sessions.Add(sess)

	// 在 session 中存储协议连接（扩展 Session 结构体）
	log.Printf("New connection: Session %s", sess.ID)

	// Start loops
	go s.writePump(sess)
	go s.readPump(sess, wsConn)
}

// readPump 使用二进制协议读取消息
func (s *Server) readPump(sess *session.Session, wsConn *protocol.WSConn) {
	defer func() {
		log.Printf("ReadPump ended for Session %s", sess.ID)
		s.sessions.Remove(sess.ID)
		sess.Conn.Close()
	}()

	sess.Conn.SetReadLimit(16 * 1024 * 1024) // 16MB max
	sess.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	sess.Conn.SetPongHandler(func(string) error {
		sess.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		// 读取二进制协议数据包
		pkt, err := wsConn.ReadPacket()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error for Session %s: %v", sess.ID, err)
			}
			break
		}

		log.Printf("ReadPump: Session %s received packet: Route=%d, Seq=%d, PayloadLen=%d",
			sess.ID, pkt.Route, pkt.Sequence, len(pkt.Payload))

		// 重置读取超时
		sess.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 路由消息 - 只传递 Payload (纯 Protobuf)
		if err := s.router.RoutePacket(sess, pkt); err != nil {
			log.Printf("Routing error for Session %s: %v", sess.ID, err)
		}
	}
}

// writePump 使用二进制协议发送消息
func (s *Server) writePump(sess *session.Session) {
	ticker := time.NewTicker(50 * time.Second) // Ping period
	defer func() {
		ticker.Stop()
		sess.Conn.Close()
		log.Printf("WritePump ended for Session %s", sess.ID)
	}()

	for {
		select {
		case message, ok := <-sess.Send:
			sess.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				sess.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// message 现在是完整的协议包（已包含头部）
			// 直接通过 WebSocket 发送
			log.Printf("WritePump: Sending %d bytes to Session %s", len(message), sess.ID)

			if err := sess.Conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
				log.Printf("WritePump: Write error for Session %s: %v", sess.ID, err)
				return
			}

			log.Printf("WritePump: Successfully sent to Session %s", sess.ID)

		case <-ticker.C:
			sess.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := sess.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
