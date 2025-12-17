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
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("Connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

    // Start Response Reader
    go func() {
        for {
            _, message, err := c.ReadMessage()
            if err != nil {
                log.Println("read:", err)
                return
            }
            log.Printf("Received: %d bytes", len(message))
            
            // Unmarshal Envelope
            var envelope gateway.Envelope
            if err := proto.Unmarshal(message, &envelope); err != nil {
                log.Printf("Unmarshal envelope failed: %v", err)
                continue
            }
            log.Printf("Envelope: Route=%v, Sequence=%d, PayloadLen=%d", envelope.Route, envelope.Sequence, len(envelope.Payload))
            
            // Unmarshal Payload (Expect ChatResponse)
            if envelope.Route == gateway.Envelope_CHAT {
                 // Wait, response type?
                 // GCS sends `ChatResponse` in payload? Or generic `ChatResponse`?
                 // Our test_client integration plan implies bidirectional.
                 // GCS sends `ChatResponse`.
                 // Let's unzip it.
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
            SessionId: "will-be-overwritten", // Gateway doesn't use this for routing TO client, but FROM client? 
            // Gateway assigns SessionID on connection.
        },
        ReceiverId: 1002,
        Content: "Hello World via Gateway!",
        Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(chatReq)

    // 2. Construct Envelope
    envelope := &gateway.Envelope{
        Route: gateway.Envelope_CHAT,
        GameId: "mmo",
        Sequence: 1,
        Payload: payload,
    }
    data, _ := proto.Marshal(envelope)

    // 3. Send
    log.Printf("Sending message...")
	err = c.WriteMessage(websocket.BinaryMessage, data)
	if err != nil {
		log.Fatal("write:", err)
	}
    
    // 4. Wait for response
    time.Sleep(2 * time.Second)
    log.Println("Done.")
}
