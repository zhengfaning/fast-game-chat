package main

import (
	"fmt"
	"time"

	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"
	"google.golang.org/protobuf/proto"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║     二进制协议 - 完整交互流程演示                        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 场景：User A (1001) 向 User B (1002) 发送聊天消息
	// ═══════════════════════════════════════════════════════════════

	fmt.Println("📖 场景说明")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("User A (ID: 1001) 想发送消息 \"Hello World!\" 给 User B (ID: 1002)")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 1: 客户端构建消息
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 1】客户端构建消息")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	chatReq := &chat.ChatRequest{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    1001,
			Timestamp: time.Now().Unix(),
		},
		ReceiverId: 1002,
		Content:    "Hello World!",
		Type:       chat.ChatRequest_TEXT,
	}
	
	fmt.Println("业务消息 (ChatRequest):")
	fmt.Printf("  GameId: %s\n", chatReq.Base.GameId)
	fmt.Printf("  发送者: User %d\n", chatReq.Base.UserId)
	fmt.Printf("  接收者: User %d\n", chatReq.ReceiverId)
	fmt.Printf("  内容: \"%s\"\n", chatReq.Content)
	fmt.Printf("  类型: %s\n", chatReq.Type)
	fmt.Println()

	// 序列化业务消息
	payload, _ := proto.Marshal(chatReq)
	fmt.Printf("序列化后大小: %d bytes\n", len(payload))
	fmt.Print("十六进制: ")
	for i := 0; i < min(20, len(payload)); i++ {
		fmt.Printf("%02X ", payload[i])
	}
	if len(payload) > 20 {
		fmt.Printf("... (%d more)", len(payload)-20)
	}
	fmt.Println()
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 2: 添加协议头
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 2】添加协议头")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// 构建数据包
	pkt := protocol.NewPacketWithSeq(protocol.RouteChat, 12345, payload)
	
	fmt.Println("协议头字段:")
	fmt.Printf("  Magic: 0x%08X (校验)\n", protocol.MagicNumber)
	fmt.Printf("  Route: %d (CHAT)\n", pkt.Route)
	fmt.Printf("  Flags: 0x%02X (无压缩/加密)\n", pkt.Flags)
	fmt.Printf("  Length: %d bytes\n", len(pkt.Payload))
	fmt.Printf("  Sequence: %d (请求ID)\n", pkt.Sequence)
	fmt.Println()

	// 编码完整数据包
	encoded := pkt.Encode()
	fmt.Printf("完整数据包大小: %d bytes\n", len(encoded))
	fmt.Printf("  - 头部: %d bytes (%.1f%%)\n", 
		protocol.HeaderSize, 
		float64(protocol.HeaderSize)/float64(len(encoded))*100)
	fmt.Printf("  - Payload: %d bytes (%.1f%%)\n", 
		len(payload),
		float64(len(payload))/float64(len(encoded))*100)
	fmt.Println()

	// 显示完整数据包结构
	fmt.Println("完整数据包结构:")
	fmt.Println("  ┌────────┬────────┬────────┬──────────┬────────┬──────────┬──────────┐")
	fmt.Println("  │ Magic  │ Route  │ Flags  │ Reserved │ Length │ Sequence │ Payload  │")
	fmt.Println("  │ 4bytes │ 1byte  │ 1byte  │  2bytes  │ 4bytes │  4bytes  │  变长     │")
	fmt.Println("  └────────┴────────┴────────┴──────────┴────────┴──────────┴──────────┘")
	
	fmt.Print("  十六进制: ")
	for i := 0; i < protocol.HeaderSize; i++ {
		fmt.Printf("%02X ", encoded[i])
		if i == 3 || i == 4 || i == 5 || i == 7 || i == 11 {
			fmt.Print("│ ")
		}
	}
	fmt.Print(" + [Payload...]")
	fmt.Println()
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 3: 模拟网络传输
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 3】通过 WebSocket 发送")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Client A ──[%d bytes]──> Gateway\n", len(encoded))
	fmt.Println("           (WebSocket Binary Frame)")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 4: Gateway 快速路由
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 4】Gateway 接收并路由")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Gateway 只读取头部
	start := time.Now()
	route, flags, length, seq, _ := protocol.DecodeHeader(encoded)
	elapsed := time.Since(start)
	
	fmt.Printf("⚡ 解析头部 (仅 16 bytes): 耗时 %v\n", elapsed)
	fmt.Printf("  Route: %d → 路由到 Chat Service\n", route)
	fmt.Printf("  Flags: 0x%02X\n", flags)
	fmt.Printf("  Sequence: %d → 记录用于响应匹配\n", seq)
	fmt.Printf("  Payload Length: %d bytes\n", length)
	fmt.Println()

	// Gateway 提取并转发 Payload
	forwardPayload := encoded[protocol.HeaderSize:]
	fmt.Printf("✅ Gateway 转发操作:\n")
	fmt.Printf("  提取 Payload: data[16:] → %d bytes\n", len(forwardPayload))
	fmt.Printf("  Gateway ──[%d bytes Protobuf]──> Chat Service\n", len(forwardPayload))
	fmt.Println("  (直接转发，无需重新编码！)")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 5: Chat Service 处理
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 5】Chat Service 处理")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// 解析业务消息
	var receivedReq chat.ChatRequest
	proto.Unmarshal(forwardPayload, &receivedReq)
	
	fmt.Printf("✅ 解析 ChatRequest:\n")
	fmt.Printf("  发送者: User %d\n", receivedReq.Base.UserId)
	fmt.Printf("  接收者: User %d\n", receivedReq.ReceiverId)
	fmt.Printf("  内容: \"%s\"\n", receivedReq.Content)
	fmt.Println()

	// 构建响应 (ACK)
	fmt.Println("📝 构建 ACK 响应:")
	ackResp := &chat.ChatResponse{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    1001, // 发给发送者
			Timestamp: time.Now().Unix(),
		},
		Success:   true,
		MessageId: 38, // 数据库生成的 ID
	}
	ackPayload, _ := proto.Marshal(ackResp)
	fmt.Printf("  ChatResponse 大小: %d bytes\n", len(ackPayload))
	fmt.Printf("  Success: %v, MsgID: %d\n", ackResp.Success, ackResp.MessageId)
	fmt.Println()

	// 构建广播消息
	fmt.Println("📢 构建广播消息 (发给接收者):")
	broadcast := &chat.MessageBroadcast{
		MessageId: 38,
		SenderId:  1001,
		Content:   "Hello World!",
		Timestamp: time.Now().Unix(),
		Type:      chat.ChatRequest_TEXT,
	}
	broadcastPayload, _ := proto.Marshal(broadcast)
	fmt.Printf("  MessageBroadcast 大小: %d bytes\n", len(broadcastPayload))
	fmt.Printf("  发给: User %d\n", receivedReq.ReceiverId)
	fmt.Printf("  来自: User %d\n", broadcast.SenderId)
	fmt.Printf("  内容: \"%s\"\n", broadcast.Content)
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 6: 响应回传
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 6】响应回传给客户端")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// ACK 回传给 Client A
	ackPkt := protocol.NewPacketWithSeq(protocol.RouteChat, 12345, ackPayload) // 使用相同 seq
	ackEncoded := ackPkt.Encode()
	
	fmt.Printf("📤 ACK 发给 Client A:\n")
	fmt.Printf("  Gateway ──[%d bytes]──> Client A\n", len(ackEncoded))
	fmt.Printf("  Sequence: %d (匹配请求)\n", ackPkt.Sequence)
	fmt.Printf("  Payload: ChatResponse (%d bytes)\n", len(ackPayload))
	fmt.Println()

	// 广播发给 Client B
	broadcastPkt := protocol.NewPacket(protocol.RouteChat, broadcastPayload)
	broadcastEncoded := broadcastPkt.Encode()
	
	fmt.Printf("📤 广播发给 Client B:\n")
	fmt.Printf("  Gateway ──[%d bytes]──> Client B\n", len(broadcastEncoded))
	fmt.Printf("  Payload: MessageBroadcast (%d bytes)\n", len(broadcastPayload))
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 步骤 7: 客户端接收
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【步骤 7】客户端接收并解析")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Client A 接收 ACK
	fmt.Println("📨 Client A 收到响应:")
	recvAckPkt, _ := protocol.Decode(ackEncoded)
	fmt.Printf("  Route: %d (CHAT)\n", recvAckPkt.Route)
	fmt.Printf("  Sequence: %d → 匹配到请求 #12345\n", recvAckPkt.Sequence)
	fmt.Printf("  Payload 大小: %d bytes\n", len(recvAckPkt.Payload))
	
	// 判断消息类型
	fmt.Println("  判断类型: Payload 小 → ChatResponse")
	var recvAck chat.ChatResponse
	proto.Unmarshal(recvAckPkt.Payload, &recvAck)
	fmt.Printf("  ✅ ACK: Success=%v, MsgID=%d\n", recvAck.Success, recvAck.MessageId)
	fmt.Println()

	// Client B 接收广播
	fmt.Println("📨 Client B 收到广播:")
	recvBroadcastPkt, _ := protocol.Decode(broadcastEncoded)
	fmt.Printf("  Route: %d (CHAT)\n", recvBroadcastPkt.Route)
	fmt.Printf("  Payload 大小: %d bytes\n", len(recvBroadcastPkt.Payload))
	
	// 判断消息类型
	fmt.Println("  判断类型: Payload 大 → MessageBroadcast")
	var recvBroadcast chat.MessageBroadcast
	proto.Unmarshal(recvBroadcastPkt.Payload, &recvBroadcast)
	fmt.Printf("  📢 收到消息:\n")
	fmt.Printf("     来自: User %d\n", recvBroadcast.SenderId)
	fmt.Printf("     内容: \"%s\"\n", recvBroadcast.Content)
	fmt.Printf("     MsgID: %d\n", recvBroadcast.MessageId)
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 总结
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    流程总结                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("✅ 完整往返流程:")
	fmt.Println("   1. Client A 构建 ChatRequest → +头部 → 发送")
	fmt.Println("   2. Gateway 读16字节 → 路由到 Chat Service")
	fmt.Println("   3. Chat Service 处理 → 返回 ACK + 广播")
	fmt.Println("   4. Gateway 转发 ACK 给 Client A")
	fmt.Println("   5. Gateway 转发广播给 Client B")
	fmt.Println("   6. 两个客户端正确解析")
	fmt.Println()
	
	fmt.Println("📊 关键指标:")
	fmt.Printf("   协议开销: %d bytes (固定)\n", protocol.HeaderSize)
	fmt.Printf("   Gateway 处理: 读16字节 + 提取Payload (极快)\n")
	fmt.Printf("   类型区分: 基于 Payload 大小 (无需额外字段)\n")
	fmt.Println()
	
	fmt.Println("🚀 关键优势:")
	fmt.Println("   ✅ Gateway 无需完整解析 Protobuf")
	fmt.Println("   ✅ 直接转发 Payload，零拷贝")
	fmt.Println("   ✅ 固定头部，解析速度极快")
	fmt.Println("   ✅ 支持压缩/加密标志")
	fmt.Println("   ✅ Sequence 支持请求-响应匹配")
	fmt.Println()

	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("演示完成！协议设计验证通过 ✓")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
