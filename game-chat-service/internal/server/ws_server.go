package server

import (
	"context"
	"log"
	"net/http"

	"game-chat-service/internal/service"
	"game-protocols/chat"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type WSServer struct {
	addr     string
	svc      *service.ChatService
	upgrader websocket.Upgrader
}

func NewWSServer(addr string, svc *service.ChatService) *WSServer {
	return &WSServer{
		addr: addr,
		svc:  svc,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for dev
			},
		},
	}
}

func (s *WSServer) Start() error {
	http.HandleFunc("/", s.handleConnection)
	log.Printf("Chat WS Server listening on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *WSServer) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("Gateway connected")

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}

		// Parse ChatRequest
		var chatReq chat.ChatRequest
		if err := proto.Unmarshal(message, &chatReq); err != nil {
			log.Printf("Failed to unmarshal ChatRequest: %v", err)
			continue
		}

		// Process request
		resp, err := s.svc.HandleRequest(context.Background(), &chatReq)
		if err != nil {
			log.Printf("HandleRequest error: %v", err)
			resp = &chat.ChatResponse{
				Base:         chatReq.Base,
				Success:      false,
				ErrorMessage: err.Error(),
			}
		}

		// Send response back
		respData, err := proto.Marshal(resp)
		if err != nil {
			log.Printf("Marshal response error: %v", err)
			continue
		}

		if err := conn.WriteMessage(websocket.BinaryMessage, respData); err != nil {
			log.Printf("Write error: %v", err)
			break
		}
	}
}
