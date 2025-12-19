package client

import (
	"fmt"
	"log"
	"net/url"
	"sync/atomic"
	"time"

	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"
	"stress_go/model"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

// GameChatClient 游戏聊天客户端
type GameChatClient struct {
	userID int32
	conn   *protocol.WSConn
	seq    uint32
	debug  bool
}

// NewGameChatClient 创建新的游戏聊天客户端
func NewGameChatClient(userID int32, gatewayURL string, debug bool) (*GameChatClient, error) {
	u, err := url.Parse(gatewayURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial failed: %w", err)
	}

	return &GameChatClient{
		userID: userID,
		conn:   protocol.NewWSConn(c),
		seq:    0,
		debug:  debug,
	}, nil
}

// Close 关闭连接
func (c *GameChatClient) Close() error {
	return c.conn.Close()
}

// Bind 绑定用户
func (c *GameChatClient) Bind() error {
	bindReq := &chat.ChatRequest{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    c.userID,
			Timestamp: time.Now().Unix(),
		},
		ReceiverId: c.userID,
		Content:    "bind",
		Type:       chat.ChatRequest_TEXT,
	}

	payload, err := proto.Marshal(bindReq)
	if err != nil {
		return fmt.Errorf("marshal bind request failed: %w", err)
	}

	_, err = c.conn.SendRequest(protocol.RouteChat, payload)
	if err != nil {
		return fmt.Errorf("send bind request failed: %w", err)
	}
	atomic.AddUint32(&c.seq, 1)

	if c.debug {
		log.Printf("[User %d] Sent Bind request", c.userID)
	}

	// 等待 Bind ACK
	// 在高并发下，可能会先收到其他人的广播消息，我们需要循环读取直到收到响应
	deadline := time.Now().Add(10 * time.Second)
	c.conn.SetReadDeadline(deadline)

	for {
		pkt, err := c.conn.ReadPacket()
		if err != nil {
			return fmt.Errorf("read bind ack failed: %w", err)
		}

		// 尝试解析为 ACK
		var resp chat.ChatResponse
		if err := proto.Unmarshal(pkt.Payload, &resp); err == nil && resp.Success {
			if c.debug {
				log.Printf("[User %d] ✅ Bind successful", c.userID)
			}
			return nil
		}

		// 如果收到的是广播消息，在 Bind 阶段暂时忽略，继续等待 ACK
		if time.Now().After(deadline) {
			return fmt.Errorf("bind timeout while waiting for ACK")
		}
	}
}

// SendMessage 发送消息
func (c *GameChatClient) SendMessage(targetID int32, content string) error {
	sendReq := &chat.ChatRequest{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    c.userID,
			Timestamp: time.Now().Unix(),
		},
		ReceiverId: targetID,
		Content:    content,
		Type:       chat.ChatRequest_TEXT,
	}

	payload, err := proto.Marshal(sendReq)
	if err != nil {
		return fmt.Errorf("marshal send request failed: %w", err)
	}

	_, err = c.conn.SendRequest(protocol.RouteChat, payload)
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	atomic.AddUint32(&c.seq, 1)

	if c.debug {
		log.Printf("[User %d] 📤 Sent message to %d: %s", c.userID, targetID, content)
	}

	return nil
}

// ReceiveMessages 接收消息
func (c *GameChatClient) ReceiveMessages(expectedCount int, timeout time.Duration) (int, error) {
	receivedCount := 0
	deadline := time.Now().Add(timeout)
	c.conn.SetReadDeadline(deadline)

	for receivedCount < expectedCount {
		pkt, err := c.conn.ReadPacket()
		if err != nil {
			if receivedCount > 0 {
				return receivedCount, fmt.Errorf("partial receive (%d/%d): %w", receivedCount, expectedCount, err)
			}
			return 0, fmt.Errorf("receive failed: %w", err)
		}

		if pkt.Route != protocol.RouteChat {
			continue
		}

		// 尝试解析为 Broadcast
		var bc chat.MessageBroadcast
		if err := proto.Unmarshal(pkt.Payload, &bc); err == nil && bc.Content != "" && bc.SenderId != c.userID {
			receivedCount++
			if c.debug {
				log.Printf("[User %d] 📨 Broadcast from %d: %s", c.userID, bc.SenderId, bc.Content)
			}
			continue
		}

		// 尝试解析为 ACK
		var resp chat.ChatResponse
		if err := proto.Unmarshal(pkt.Payload, &resp); err == nil && resp.Success {
			receivedCount++
			if c.debug {
				log.Printf("[User %d] ✅ ACK received", c.userID)
			}
			continue
		}
	}

	return receivedCount, nil
}

// RunTest 运行单次测试
func (c *GameChatClient) RunTest(numMessages int, startUserID int32, concurrency uint64) *model.RequestResult {
	result := &model.RequestResult{
		UserID:  c.userID,
		Success: false,
	}

	startTime := time.Now()

	// 1. 计算目标 ID (形成一个环: userID -> userID + 1)
	totalUsers := int32(concurrency)
	relativeID := c.userID - startUserID
	nextRelativeID := (relativeID + 1) % totalUsers
	targetID := startUserID + nextRelativeID

	if c.debug {
		log.Printf("[User %d] Targeting User %d", c.userID, targetID)
	}

	// 2. 循环执行：发送消息 -> 接收响应
	for i := 0; i < numMessages; i++ {
		content := fmt.Sprintf("Test message %d from %d", i+1, c.userID)

		// 发送
		if err := c.SendMessage(targetID, content); err != nil {
			result.Error = fmt.Errorf("send message %d failed: %w", i+1, err)
			result.Duration = time.Since(startTime)
			return result
		}
		result.MessagesSent++

		// 接收与等待
		received, err := c.ReceiveMessages(1, 10*time.Second)
		result.MessagesRecv += received

		if err != nil {
			result.Error = fmt.Errorf("receive response for message %d failed: %w", i+1, err)
			result.Duration = time.Since(startTime)
			return result
		}
	}

	result.Success = true
	result.Duration = time.Since(startTime)

	return result
}
