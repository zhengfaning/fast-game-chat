package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"
)

// 配置
const (
	UserCount      = 1000
	StartUserID    = 2000
	ConnectTimeout = 10 * time.Second
	RedisAddr      = "localhost:6379"
)

var (
	rdb *redis.Client
	ctx = context.Background()
)

// 初始化 Redis
func initRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr: RedisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Redis connection failed: %v", err)
	}
	// 清理旧数据
	rdb.Del(ctx, "stress:sent", "stress:recv")
}

func connect(userID int32) (*protocol.WSConn, error) {
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, err
	}
	return protocol.NewWSConn(c), nil
}

// 模拟单个用户行为
func runUser(id int32, readyWg *sync.WaitGroup, finishWg *sync.WaitGroup, startChatChan chan bool) {
	defer finishWg.Done()
	conn, err := connect(id)
	if err != nil {
		log.Printf("[User %d] Connect failed: %v", id, err)
		readyWg.Done() // 防止死锁
		return
	}
	defer conn.Close()

	// 1. Bind
	bindReq := &chat.ChatRequest{
		Base:       &common.MessageBase{GameId: "mmo", UserId: id, Timestamp: time.Now().Unix()},
		ReceiverId: id, Content: "bind", Type: chat.ChatRequest_TEXT,
	}
	payload, _ := proto.Marshal(bindReq)
	conn.SendRequest(protocol.RouteChat, payload)

	conn.SetReadLimit(int64(protocol.MaxPacketSize))
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	// 等待 Bind ACK
	_, err = conn.ReadPacket()
	if err != nil {
		log.Printf("[User %d] Bind failed (read): %v", id, err)
		readyWg.Done()
		return
	}

	log.Printf("[User %d] Ready", id)
	readyWg.Done() // 通知已就绪

	// 等待所有用户连接完成
	select {
	case <-startChatChan:
	case <-time.After(15 * time.Second):
		log.Printf("[User %d] Timeout waiting for start signal", id)
		return
	}

	// 2. 发送消息逻辑 (圆环模式: 我 -> 下一个人)
	targetID := id + 1
	if targetID >= StartUserID+UserCount {
		targetID = StartUserID
	}

	// 唯一消息内容
	msgContent := fmt.Sprintf("UUID-%d-TO-%d-%d", id, targetID, time.Now().UnixNano())

	// 记录期望到 Redis (Sender:Target:Content)
	recordKey := fmt.Sprintf("%d:%d:%s", id, targetID, msgContent)
	rdb.SAdd(ctx, "stress:sent", recordKey)

	// log.Printf("[User %d] Sending to %d", id, targetID)

	sendReq := &chat.ChatRequest{
		Base:       &common.MessageBase{GameId: "mmo", UserId: id, Timestamp: time.Now().Unix()},
		ReceiverId: targetID,
		Content:    msgContent,
		Type:       chat.ChatRequest_TEXT,
	}
	payload, _ = proto.Marshal(sendReq)
	conn.SendRequest(protocol.RouteChat, payload)

	// 3. 接收循环
	// 我们期望收到:
	// 1. 发送消息的 ACK
	// 2. 上一个人发给我的 Broadcast

	expectedEvents := 2 // ACK + Broadcast
	receivedAck := false
	receivedBroadcast := false

	timeout := time.After(10 * time.Second)

	for expectedEvents > 0 {
		select {
		case <-timeout:
			log.Printf("[User %d] Timeout waiting for (ACK=%v, Broadcast=%v)", id, receivedAck, receivedBroadcast)
			return
		default:
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			pkt, err := conn.ReadPacket()
			if err != nil {
				continue
			}

			if pkt.Route != protocol.RouteChat {
				continue
			}

			// Try Broadcast
			var bc chat.MessageBroadcast
			if err := proto.Unmarshal(pkt.Payload, &bc); err == nil && bc.Content != "" && bc.SenderId != id {
				// log.Printf("[User %d] Received from %d", id, bc.SenderId)

				// 记录实际接收到 Redis (Sender:Target:Content)
				recvKey := fmt.Sprintf("%d:%d:%s", bc.SenderId, id, bc.Content)
				rdb.SAdd(ctx, "stress:recv", recvKey)

				if !receivedBroadcast {
					receivedBroadcast = true
					expectedEvents--
				}
				continue
			}

			// Try ACK
			var resp chat.ChatResponse
			if err := proto.Unmarshal(pkt.Payload, &resp); err == nil && resp.Success {
				if !receivedAck {
					receivedAck = true
					expectedEvents--
				}
				continue
			}
		}
	}
}

func main() {
	log.Println("=== Starting Stress Test (20 Users) ===")
	log.Println("STEP 1: Initializing Redis and Connections...")
	initRedis()

	var readyWg sync.WaitGroup
	var finishWg sync.WaitGroup
	readyWg.Add(UserCount)
	finishWg.Add(UserCount)
	startChan := make(chan bool)

	// 启动所有用户
	for i := 0; i < UserCount; i++ {
		uid := int32(StartUserID + i)
		go runUser(uid, &readyWg, &finishWg, startChan)
		time.Sleep(10 * time.Millisecond) // 稍微错开连接风暴
	}

	log.Println("Waiting for all users to connect...")
	readyWg.Wait()
	log.Println("✅ All users connected and bound!")

	// 触发聊天阶段
	log.Println("STEP 2: Starting mutual communication...")
	close(startChan)

	// 等待所有用户完成交互
	finishWg.Wait()
	log.Println("Simulation finished. Verifying data...")

	// 验证
	verify()
}

func verify() {
	sentCount, err := rdb.SCard(ctx, "stress:sent").Result()
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}
	recvCount, err := rdb.SCard(ctx, "stress:recv").Result()
	if err != nil {
		log.Fatalf("Redis error: %v", err)
	}

	log.Printf("Messages Sent: %d", sentCount)
	log.Printf("Messages Received: %d", recvCount)

	if sentCount == 0 {
		log.Fatal("❌ No messages sent!")
	}

	// 如果发送数 != 接收数，肯定是丢了
	if sentCount != int64(UserCount) {
		log.Printf("⚠️ WARNING: Expected %d sent messages, got %d", UserCount, sentCount)
	}

	// 找出丢失的消息
	// SDiff: 返回在 Sent 但不在 Recv 的元素
	diff, err := rdb.SDiff(ctx, "stress:sent", "stress:recv").Result()
	if err != nil {
		log.Fatalf("Redis SDiff error: %v", err)
	}

	if len(diff) == 0 && sentCount == recvCount {
		log.Println("🎉 VERIFICATION SUCCESS: All messages sent were received correctly.")
	} else {
		log.Printf("❌ VERIFICATION FAILED: %d messages lost:", len(diff))
		for _, msg := range diff {
			log.Printf("   - Lost: %s", msg)
		}
		panic("Verification Failed")
	}
}
