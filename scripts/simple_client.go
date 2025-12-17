package main

import (
"log"
"net/url"
"time"

    "game-protocols/gateway"
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
    
    log.Println("Sending message...")
    req := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, Content: "Test", Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(req)
    env := &gateway.Envelope{Route: gateway.Envelope_CHAT, GameId: "mmo", UserId: 1002, Payload: payload}
    data, _ := proto.Marshal(env)
    c.WriteMessage(websocket.BinaryMessage, data)
    
    log.Println("Reading responses...")
    for i := 0; i < 5; i++ {
        c.SetReadDeadline(time.Now().Add(2 * time.Second))
        _, msg, err := c.ReadMessage()
        if err != nil {
            log.Printf("Read error #%d: %v", i+1, err)
            break
        }
        log.Printf("Received message #%d, len=%d", i+1, len(msg))
    }
    
    log.Println("Done")
}
