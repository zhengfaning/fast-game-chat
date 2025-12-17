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

var successB = false
var successA = false

func connect(name string, userID int32) (*protocol.WSConn, error) {
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        return nil, err
    }
    log.Printf("[%s] Connected to %s", name, u.String())
    return protocol.NewWSConn(c), nil
}

// 模拟 Client B (接收者)
func runClientB(done chan bool) {
    name := "Client B"
    conn, err := connect(name, 1002)
    if err != nil {
        log.Fatalf("[%s] Connect failed: %v", name, err)
    }
    defer conn.Close()

    // 1. Bind User (发送第一条消息以建立 Session-User 映射)
    log.Printf("[%s] Sending Bind Request...", name)
    bindReq := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, Content: "Init B", Type: chat.ChatRequest_TEXT,
    }
    bindPayload, _ := proto.Marshal(bindReq)
    if _, err := conn.SendRequest(protocol.RouteChat, bindPayload); err != nil {
        log.Fatalf("[%s] Send bind failed: %v", name, err)
    }

    // 2. 读取循环
    log.Printf("[%s] Waiting for messages...", name)
    timeout := time.After(10 * time.Second)
    
    // 用于检测连接是否健康的 channel
    readChan := make(chan *protocol.Packet)
    errChan := make(chan error)

    go func() {
        for {
            // 设置每次读取的 deadline，防止永久阻塞
            conn.SetReadDeadline(time.Now().Add(10 * time.Second))
            pkt, err := conn.ReadPacket()
            if err != nil {
                errChan <- err
                return
            }
            readChan <- pkt
        }
    }()

    for {
        select {
        case <-timeout:
            log.Printf("[%s] ❌ Test Timeout!", name)
            done <- false
            return
            
        case err := <-errChan:
            log.Printf("[%s] Read Error: %v", name, err)
            done <- false
            return

        case pkt := <-readChan:
            if pkt.Route != protocol.RouteChat {
                continue
            }

            // 尝试解析为 Broadcast (这是我们期待的)
            var broadcast chat.MessageBroadcast
            // Heuristic: Broadcast content should not be empty, and SenderId should be valid
            if err := proto.Unmarshal(pkt.Payload, &broadcast); err == nil && len(pkt.Payload) > 20 {
                if broadcast.Content != "" && broadcast.SenderId > 0 {
                    log.Printf("[%s] 📨 Received Broadcast from %d: %s", name, broadcast.SenderId, broadcast.Content)
                    
                    if broadcast.SenderId == 1001 && broadcast.Content == "Hello B, I am A!" {
                        successB = true
                        log.Printf("[%s] ✅ Verified Correct Message!", name)
                        done <- true
                        return
                    }
                    // 可能是自己的 "Init B" 回显，忽略
                    continue
                }
            }
            
            // 尝试解析为 ACK
            var resp chat.ChatResponse
            if err := proto.Unmarshal(pkt.Payload, &resp); err == nil {
                log.Printf("[%s] Received ACK: Success=%v MsgID=%d", name, resp.Success, resp.MessageId)
            }
        }
    }
}

// 模拟 Client A (发送者)
func runClientA(done chan bool) {
    name := "Client A"
    // 等待 Client B 先准备好
    time.Sleep(1 * time.Second)
    
    conn, err := connect(name, 1001)
    if err != nil {
        log.Fatalf("[%s] Connect failed: %v", name, err)
    }
    defer conn.Close()

    // 1. 发送消息给 B
    log.Printf("[%s] Sending Message to Client B...", name)
    msgReq := &chat.ChatRequest{
        Base: &common.MessageBase{GameId: "mmo", UserId: 1001, Timestamp: time.Now().Unix()},
        ReceiverId: 1002, // Target User 1002 (Client B)
        Content: "Hello B, I am A!",
        Type: chat.ChatRequest_TEXT,
    }
    payload, _ := proto.Marshal(msgReq)
    if _, err := conn.SendRequest(protocol.RouteChat, payload); err != nil {
        log.Fatalf("[%s] Send message failed: %v", name, err)
    }

    // 2. 等待 ACK
    conn.SetReadDeadline(time.Now().Add(5 * time.Second))
    pkt, err := conn.ReadPacket()
    if err != nil {
        log.Printf("[%s] Failed to receive ACK: %v", name, err)
        done <- false
        return
    }

    var resp chat.ChatResponse
    if err := proto.Unmarshal(pkt.Payload, &resp); err == nil && resp.Success {
        log.Printf("[%s] ✅ Received ACK: MsgID=%d", name, resp.MessageId)
        successA = true
        done <- true
    } else {
        log.Printf("[%s] ❌ Received invalid response", name)
        done <- false
    }
}

func main() {
    log.Println("=== Starting Realistic Pair Test ===")
    
    doneB := make(chan bool)
    doneA := make(chan bool)

    go runClientB(doneB)
    go runClientA(doneA)

    // Wait for results
    resultB := <-doneB
    resultA := <-doneA

    log.Println("\n=== Test Results ===")
    if resultA {
        log.Println("✅ Client A: Send & ACK OK")
    } else {
        log.Println("❌ Client A: Failed")
    }
    
    if resultB {
        log.Println("✅ Client B: Receive OK")
    } else {
        log.Println("❌ Client B: Failed")
    }

    if resultA && resultB {
        log.Println("\n🎉 REALISTIC SCENARIO TEST PASSED!")
    } else {
        log.Println("\n❌ TEST FAILED - Check logs above")
        panic("Test Failed")
    }
}
