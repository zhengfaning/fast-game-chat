package main

import (
	"context"
	"flag"
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
	StartUserID    = 2000
	ConnectTimeout = 10 * time.Second
	RedisAddr      = "192.168.31.35:6379"
)

var (
	UserCount int // 从命令行参数读取
	rdb       *redis.Client
	ctx       = context.Background()
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

	// 添加 panic 恢复，防止单个 goroutine 崩溃影响整体测试
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[User %d] Recovered from panic: %v", id, r)
		}
	}()

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

	// 等待所有用户连接完成（超时 = 用户数 × 连接间隔 + 缓冲时间）
	// 连接间隔 10ms，缓冲 30秒
	waitTimeout := time.Duration(UserCount)*10*time.Millisecond + 30*time.Second
	select {
	case <-startChatChan:
	case <-time.After(waitTimeout):
		log.Printf("[User %d] Timeout waiting for start signal (waited %v)", id, waitTimeout)
		return
	}

	// 2. 发送消息逻辑 (圆环模式: 我 -> 下一个人)
	targetID := id + 1
	if targetID >= int32(StartUserID+UserCount) {
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

	// 超时 10 分钟（足够长，确保高并发下也能收到消息）
	// 超时 10 分钟（足够长，确保高并发下也能收到消息）
	log.Printf("[User %d] Waiting for messages (timeout: 10min)...", id)

	conn.SetReadDeadline(time.Now().Add(10 * time.Minute))

	for expectedEvents > 0 {
		pkt, err := conn.ReadPacket()
		if err != nil {
			log.Printf("[User %d] ❌ Read error (fatal): %v", id, err)
			return
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
				log.Printf("[User %d] ✅ Broadcast received from %d | Remaining: %d", id, bc.SenderId, expectedEvents)
			}

			// 如果已经收到所有消息，立即退出
			if expectedEvents == 0 {
				log.Printf("[User %d] ✅ All messages received, closing connection", id)
				return
			}
			continue
		}

		// Try ACK
		var resp chat.ChatResponse
		if err := proto.Unmarshal(pkt.Payload, &resp); err == nil && resp.Success {
			if !receivedAck {
				receivedAck = true
				expectedEvents--
				log.Printf("[User %d] ✅ ACK received | Remaining: %d", id, expectedEvents)
			}

			// 如果已经收到所有消息，立即退出
			if expectedEvents == 0 {
				log.Printf("[User %d] ✅ All messages received, closing connection", id)
				return
			}
			continue
		}
	}
}

func main() {
	// 解析命令行参数
	flag.IntVar(&UserCount, "users", 1000, "并发用户数量")
	flag.Parse()

	log.Printf("=== Starting Stress Test (%d Users) ===", UserCount)
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
