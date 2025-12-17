package main

import (
    "log"
    "net/url"
    "time"
    "sync"

    "game-gateway/pkg/protocol"  // 引入新协议包
    "game-protocols/common"
    "game-protocols/chat"
    "github.com/gorilla/websocket"
    "google.golang.org/protobuf/proto"
)

func main() {
    var wg sync.WaitGroup
    var successA, successB bool
    
    // === CLIENT B (Receiver) ===
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        log.Println("👤 [B] Connecting...")
        u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
        connB, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
        if err != nil {
            log.Fatal("[B] Connect failed:", err)
        }
        defer connB.Close()
        
        // 包装新协议
        wsConn := protocol.NewWSConn(connB)
        
        // Bind (Send ChatRequest to bind session to UserID 1002)
        req := &chat.ChatRequest{
            Base: &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
            ReceiverId: 1002, Content: "Init B", Type: chat.ChatRequest_TEXT,
        }
        payload, _ := proto.Marshal(req)
        
        // 使用新协议发送
        wsConn.SendRequest(protocol.RouteChat, payload)
        
        log.Println("👤 [B] Bound to User 1002, waiting for messages...")
        
        // Read messages
        timeout := time.After(10 * time.Second)
        msgCount := 0
        for i := 0; i < 20; i++ {
            connB.SetReadDeadline(time.Now().Add(1 * time.Second))
            pkt, err := wsConn.ReadPacket()
            
            if err != nil {
                // Check if timeout
                select {
                case <-timeout:
                    log.Printf("👤 [B] Timeout after receiving %d messages", msgCount)
                    return
                default:
                    // log.Printf("Read error or timeout: %v", err)
                    continue
                }
            }
            
            msgCount++
            log.Printf("👤 [B] <<< Received Packet #%d, Route=%d, Len=%d", msgCount, pkt.Route, len(pkt.Payload))
            
            // Distinguish by payload length & content
            if len(pkt.Payload) > 25 {
                // Likely a Broadcast
                var bc chat.MessageBroadcast
                if err := proto.Unmarshal(pkt.Payload, &bc); err == nil {
                    log.Printf("👤 [B] 📨 BROADCAST from User %d: \"%s\" (MsgID=%d)", bc.SenderId, bc.Content, bc.MessageId)
                    if bc.SenderId == 1001 && bc.Content == "Hello B, I am A!" {
                        successB = true
                        log.Println("👤 [B] ✅ SUCCESS! Got the broadcast message!")
                        return
                    }
                }
            } else {
                // Likely an ACK
                var ack chat.ChatResponse
                if err := proto.Unmarshal(pkt.Payload, &ack); err == nil {
                     log.Printf("👤 [B] Got ACK MsgID=%d, Success=%v", ack.MessageId, ack.Success)
                }
            }
        }
    }()
    
    time.Sleep(800 * time.Millisecond) // Let B bind first
    
    // === CLIENT A (Sender) ===
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        log.Println("👤 [A] Connecting...")
        u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
        connA, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
        if err != nil {
            log.Fatal("[A] Connect failed:", err)
        }
        defer connA.Close()
        wsConn := protocol.NewWSConn(connA)
        
        log.Println("👤 [A] Connected as User 1001")
        
        // Send message to B
        req := &chat.ChatRequest{
            Base: &common.MessageBase{GameId: "mmo", UserId: 1001, Timestamp: time.Now().Unix()},
            ReceiverId: 1002,
            Content: "Hello B, I am A!",
            Type: chat.ChatRequest_TEXT,
        }
        payload, _ := proto.Marshal(req)
        
        seq, _ := wsConn.SendRequest(protocol.RouteChat, payload)
        log.Printf("👤 [A] 📤 Sent message (seq=%d) to User 1002", seq)
        
        // Wait for ACK
        connA.SetReadDeadline(time.Now().Add(2 * time.Second))
        pkt, err := wsConn.ReadPacket()
        if err == nil {
            var ack chat.ChatResponse
            if proto.Unmarshal(pkt.Payload, &ack) == nil && ack.Success {
                log.Printf("👤 [A] ✅ Got ACK MsgID=%d - SUCCESS!", ack.MessageId)
                successA = true
            }
        }
    }()
    
    wg.Wait()
    
    log.Println("\n========== RESULTS ==========")
    if successA {
        log.Println("✅ Client A: Message sent and ACKed")
    } else {
        log.Println("❌ Client A: Failed")
    }
    
    if successB {
        log.Println("✅ Client B: Received broadcast from A")
    } else {
        log.Println("❌ Client B: Failed")
    }  
    
    if successA && successB {
        log.Println("\n🎉🎉🎉 BROADCAST TEST PASSED! 🎉🎉🎉")
    } else {
        log.Println("\n❌ Test failed")
    }
}
