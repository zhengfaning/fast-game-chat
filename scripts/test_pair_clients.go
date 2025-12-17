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

var successB = false
var successA = false

func connect(userID int32) (*websocket.Conn, error) {
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    return c, err
}

func readLoopB(name string, c *websocket.Conn, done chan bool) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("[%s] Recovered from panic: %v", name, r)
        }
        done <- true
    }()
    
    timeout := time.After(6 * time.Second)
    
    for {
        select {
        case <-timeout:
            log.Printf("[%s] Timeout reached", name)
            return
        default:
            c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
            _, message, err := c.ReadMessage()
            if err != nil {
                continue 
            }
            
            var envelope gateway.Envelope
            if err := proto.Unmarshal(message, &envelope); err != nil {
                log.Printf("[%s] Unmarshal envelope failed: %v", name, err)
                continue
            }
            
            if envelope.Route == gateway.Envelope_CHAT {
                // Try unmarshal as ChatResponse (ACK) first
                var resp chat.ChatResponse
                if err := proto.Unmarshal(envelope.Payload, &resp); err == nil && resp.Success {
                    log.Printf("[%s] ✅ ACK: MsgID=%d (ignoring, waiting for broadcast...)", name, resp.MessageId)
                    continue
                }
                
                // Try unmarshal as MessageBroadcast
                var broadcast chat.MessageBroadcast
                if err := proto.Unmarshal(envelope.Payload, &broadcast); err == nil {
                    log.Printf("[%s] 📨 BROADCAST from User %d: \"%s\"", name, broadcast.SenderId, broadcast.Content)
                    // Check if this is the message we're expecting from User 1001
                    if broadcast.SenderId == 1001 && len(broadcast.Content) > 0 && broadcast.Content == "Hello B, I am A!" {
                        successB = true
                        log.Printf("[%s] ✅ SUCCESS! Received expected message from User %d", name, broadcast.SenderId)
                        return
                    }
                    continue
                }
                
                log.Printf("[%s] Unknown payload type, len=%d", name, len(envelope.Payload))
            }
        }
    }
}

func readLoopA(name string, c *websocket.Conn, done chan bool) {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("[%s] Recovered from panic: %v", name, r)
        }
        done <- true
    }()
    
    timeout := time.After(6 * time.Second)
    
    for {
        select {
        case <-timeout:
            log.Printf("[%s] Timeout reached", name)
            return
        default:
            c.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
            _, message, err := c.ReadMessage()
            if err != nil {
                continue
            }
            
            var envelope gateway.Envelope
            if err := proto.Unmarshal(message, &envelope); err != nil {
                log.Printf("[%s] Unmarshal envelope failed: %v", name, err)
                continue
            }
            
            if envelope.Route == gateway.Envelope_CHAT {
                // Try unmarshal as ChatResponse (ACK)
                var resp chat.ChatResponse
                if err := proto.Unmarshal(envelope.Payload, &resp); err == nil && resp.Success {
                    log.Printf("[%s] ✅ ACK: MsgID=%d", name, resp.MessageId)
                    successA = true
                    log.Printf("[%s] ✅ SUCCESS! Message sent successfully", name)
                    return
                }
                
                // MessageBroadcast shouldn't come to A
                var broadcast chat.MessageBroadcast
                if err := proto.Unmarshal(envelope.Payload, &broadcast); err == nil {
                    log.Printf("[%s] 📨 BROADCAST from User %d: \"%s\" (unexpected)", name, broadcast.SenderId, broadcast.Content)
                }
            }
        }
    }
}

func main() {
    doneB := make(chan bool, 1)
    doneA := make(chan bool, 1)
    
    log.Println("=== Starting Broadcast Test ===")
    
    // 1. Connect Client B (Receiver - 1002)
    clientB, err := connect(1002)
    if err != nil {
        log.Fatal("Client B connect:", err)
    }
    defer clientB.Close()
    
    // Bind User B - send dummy message to self
    bindReqB := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, Content: "Init B", Type: chat.ChatRequest_TEXT,
    }
    bindPayloadB, _ := proto.Marshal(bindReqB)
    bindEnvB := &gateway.Envelope{Route: gateway.Envelope_CHAT, GameId: "mmo", UserId: 1002, Payload: bindPayloadB}
    dataB, _ := proto.Marshal(bindEnvB)
    clientB.WriteMessage(websocket.BinaryMessage, dataB)
    
    log.Println("👤 Client B (User 1002) connected and bound")

    // Start Reader for B - don't exit on ACK, wait for broadcast from User 1001
    go readLoopB("Client B", clientB, doneB)
    
    // Wait for B to bind
    time.Sleep(500 * time.Millisecond)

    // 2. Connect Client A (Sender - 1001)
    clientA, err := connect(1001)
    if err != nil {
        log.Fatal("Client A connect:", err)
    }
    defer clientA.Close()
    log.Println("👤 Client A (User 1001) connected")
    
    // Start Reader for A
    go readLoopA("Client A", clientA, doneA)

    // 3. User A sends message to User B
    time.Sleep(300 * time.Millisecond)
    log.Println("📤 Client A sending message to Client B...")
    
    msgReq := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1001, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, // Target B
        Content: "Hello B, I am A!",
        Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(msgReq)
    env := &gateway.Envelope{Route: gateway.Envelope_CHAT, GameId: "mmo", UserId: 1001, Payload: payload}
    data, _ := proto.Marshal(env)
    
    if err := clientA.WriteMessage(websocket.BinaryMessage, data); err != nil {
        log.Fatal("Client A write:", err)
    }
    
    log.Println("⏳ Waiting for responses...")
    
    // Wait for both clients to finish or timeout
    <-doneA
    <-doneB
    
    log.Println("\n=== Test Results ===")
    if successA {
        log.Println("✅ Client A: Message sent and acknowledged")
    } else {
        log.Println("❌ Client A: Did not receive ACK")
    }
    
    if successB {
        log.Println("✅ Client B: Received message from Client A")
    } else {
        log.Println("❌ Client B: Did not receive message from Client A")
    }
    
    if successA && successB {
        log.Println("\n🎉 BROADCAST TEST PASSED!")
    } else {
        log.Println("\n❌ BROADCAST TEST FAILED")
    }
}
