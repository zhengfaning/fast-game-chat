package main

import (
	"log"
	"net/url"
	"time"

    "game-gateway/pkg/protocol"
    "game-protocols/common"
    "game-protocols/chat"
	"github.com/gorilla/websocket"
    "google.golang.org/protobuf/proto"
)

func main() {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

    wsConn := protocol.NewWSConn(c)

    // Start Response Reader
    go func() {
        for {
            pkt, err := wsConn.ReadPacket()
            if err != nil {
                log.Println("read error:", err)
                return
            }
            log.Printf("Received Packet: Route=%d, Seq=%d, PayloadLen=%d", pkt.Route, pkt.Sequence, len(pkt.Payload))
            
            // Try identify content
            if pkt.Route == protocol.RouteChat {
                 if len(pkt.Payload) > 25 {
                     var bc chat.MessageBroadcast
                     if err := proto.Unmarshal(pkt.Payload, &bc); err == nil {
                         log.Printf(" >> Broadcast from %d: %s", bc.SenderId, bc.Content)
                     }
                 } else {
                     var resp chat.ChatResponse
                     if err := proto.Unmarshal(pkt.Payload, &resp); err == nil {
                         log.Printf(" >> ACK: Success=%v", resp.Success)
                     }
                 }
            }
        }
    }()

    // 1. Construct ChatRequest
    chatReq := &chat.ChatRequest{
        Base: &common.MessageBase{
            GameId: "mmo",
            UserId: 1001,
            Timestamp: time.Now().Unix(),
            TraceId: "test-trace-1",
            SessionId: "will-be-overwritten", 
        },
        ReceiverId: 1002,
        Content: "Hello World via Gateway (Binary)!",
        Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(chatReq)

    // 2. Send using new protocol
    log.Printf("Sending message...")
	_, err = wsConn.SendRequest(protocol.RouteChat, payload)
	if err != nil {
		log.Fatal("write:", err)
	}
    
    // 4. Wait for response
    time.Sleep(2 * time.Second)
    log.Println("Done.")
}
