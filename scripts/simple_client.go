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
    log.Println("Connecting...")
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
    c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Wrap with helper
    wsConn := protocol.NewWSConn(c)
    
    log.Println("Sending message...")
    req := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, Content: "Test", Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(req)
    
    // Send using new protocol
    if _, err := wsConn.SendRequest(protocol.RouteChat, payload); err != nil {
        log.Fatal(err)
    }
    
    log.Println("Reading responses...")
    for i := 0; i < 5; i++ {
        c.SetReadDeadline(time.Now().Add(2 * time.Second))
        pkt, err := wsConn.ReadPacket()
        if err != nil {
            log.Printf("Read error #%d: %v", i+1, err)
            break
        }
        log.Printf("Received Packet #%d, Route=%d, Len=%d", i+1, pkt.Route, len(pkt.Payload))
    }
    
    log.Println("Done")
}
