package main

import (
	"fmt"
	"time"

	"game-protocols/chat"
	"game-protocols/common"
	"google.golang.org/protobuf/proto"
)

func main() {
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║        Protobuf 路由字段演示                              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 演示 1: ChatResponse with routing fields
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【演示 1】ChatResponse - 带路由字段")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	resp := &chat.ChatResponse{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    1001,
			Timestamp: time.Now().Unix(),
		},
		Success:   true,
		MessageId: 38,
		Timestamp: time.Now().Unix(),
		
		// 🆕 新增的路由字段
		TargetUserId:    1001,  // 发给 User 1001
		TargetSessionId: "abc123",  // 或指定Session
	}

	fmt.Println("ChatResponse 内容:")
	fmt.Printf("  Success: %v\n", resp.Success)
	fmt.Printf("  MessageId: %d\n", resp.MessageId)
	fmt.Println()
	fmt.Println("✨ 路由信息:")
	fmt.Printf("  TargetUserId: %d\n", resp.TargetUserId)
	fmt.Printf("  TargetSessionId: %s\n", resp.TargetSessionId)
	fmt.Println()

	// 序列化
	respData, _ := proto.Marshal(resp)
	fmt.Printf("序列化大小: %d bytes\n", len(respData))
	fmt.Print("数据 (前24 bytes): ")
	for i := 0; i < min(24, len(respData)); i++ {
		fmt.Printf("%02X ", respData[i])
	}
	if len(respData) > 24 {
		fmt.Printf("... (%d more)", len(respData)-24)
	}
	fmt.Println()
	fmt.Println()

	// Gateway 模拟解析
	fmt.Println("🔀 Gateway 路由决策:")
	var parsedResp chat.ChatResponse
	proto.Unmarshal(respData, &parsedResp)
	
	if parsedResp.TargetUserId > 0 {
		fmt.Printf("  → 路由到 User %d\n", parsedResp.TargetUserId)
		fmt.Printf("  → 查找 sessionManager.GetByUserID(%d)\n", parsedResp.TargetUserId)
	}
	if parsedResp.TargetSessionId != "" {
		fmt.Printf("  → 或路由到 Session \"%s\"\n", parsedResp.TargetSessionId)
		fmt.Printf("  → 查找 sessionManager.Get(\"%s\")\n", parsedResp.TargetSessionId)
	}
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 演示 2: MessageBroadcast with routing fields
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【演示 2】MessageBroadcast - 带路由字段")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	broadcast := &chat.MessageBroadcast{
		MessageId:  38,
		SenderId:   1001,
		SenderName: "Alice",
		Content:    "Hello everyone!",
		Timestamp:  time.Now().Unix(),
		Type:       chat.ChatRequest_TEXT,
		
		// 🆕 新增的路由字段
		TargetUserId: 1002,  // 发给 User 1002
	}

	fmt.Println("MessageBroadcast 内容:")
	fmt.Printf("  SenderId: %d (%s)\n", broadcast.SenderId, broadcast.SenderName)
	fmt.Printf("  Content: \"%s\"\n", broadcast.Content)
	fmt.Printf("  MessageId: %d\n", broadcast.MessageId)
	fmt.Println()
	fmt.Println("✨ 路由信息:")
	fmt.Printf("  TargetUserId: %d\n", broadcast.TargetUserId)
	fmt.Println()

	// 序列化
	broadcastData, _ := proto.Marshal(broadcast)
	fmt.Printf("序列化大小: %d bytes\n", len(broadcastData))
	fmt.Print("数据 (前24 bytes): ")
	for i := 0; i < min(24, len(broadcastData)); i++ {
		fmt.Printf("%02X ", broadcastData[i])
	}
	if len(broadcastData) > 24 {
		fmt.Printf("... (%d more)", len(broadcastData)-24)
	}
	fmt.Println()
	fmt.Println()

	// Gateway 模拟解析
	fmt.Println("🔀 Gateway 路由决策:")
	var parsedBroadcast chat.MessageBroadcast
	proto.Unmarshal(broadcastData, &parsedBroadcast)
	
	if parsedBroadcast.TargetUserId > 0 {
		fmt.Printf("  → 路由到 User %d\n", parsedBroadcast.TargetUserId)
		fmt.Printf("  → session := sessionManager.GetByUserID(%d)\n", parsedBroadcast.TargetUserId)
		fmt.Println("  → if session != nil { session.Send <- packet }")
	}
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 对比：有无路由字段的大小差异
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("【对比分析】路由字段的开销")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// ChatResponse - 无路由字段
	respNoRoute := &chat.ChatResponse{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    1001,
			Timestamp: time.Now().Unix(),
		},
		Success:   true,
		MessageId: 38,
	}
	respNoRouteData, _ := proto.Marshal(respNoRoute)

	// ChatResponse - 有路由字段
	respWithRouteData, _ := proto.Marshal(resp)

	fmt.Println("ChatResponse:")
	fmt.Printf("  无路由字段: %d bytes\n", len(respNoRouteData))
	fmt.Printf("  有路由字段: %d bytes\n", len(respWithRouteData))
	fmt.Printf("  增加: %d bytes (%.1f%%)\n", 
		len(respWithRouteData)-len(respNoRouteData),
		float64(len(respWithRouteData)-len(respNoRouteData))/float64(len(respNoRouteData))*100)
	fmt.Println()

	// MessageBroadcast - 无路由字段
	bcNoRoute := &chat.MessageBroadcast{
		MessageId: 38,
		SenderId:  1001,
		Content:   "Hello everyone!",
		Timestamp: time.Now().Unix(),
	}
	bcNoRouteData, _ := proto.Marshal(bcNoRoute)

	// MessageBroadcast - 有路由字段
	bcWithRouteData, _ := proto.Marshal(broadcast)

	fmt.Println("MessageBroadcast:")
	fmt.Printf("  无路由字段: %d bytes\n", len(bcNoRouteData))
	fmt.Printf("  有路由字段: %d bytes\n", len(bcWithRouteData))
	fmt.Printf("  增加: %d bytes (%.1f%%)\n",
		len(bcWithRouteData)-len(bcNoRouteData),
		float64(len(bcWithRouteData)-len(bcNoRouteData))/float64(len(bcNoRouteData))*100)
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════
	// 总结
	// ═══════════════════════════════════════════════════════════════
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║                        总结                               ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("✅ 路由字段已成功添加到 Protobuf 定义")
	fmt.Println()
	fmt.Println("📊 开销分析:")
	fmt.Println("  - 每条消息增加 ~2-10 bytes（取决于 UserID 大小）")
	fmt.Println("  - 对于典型消息（100+ bytes），开销 < 10%")
	fmt.Println()
	fmt.Println("🚀 优势:")
	fmt.Println("  ✅ Gateway 无需维护状态映射")
	fmt.Println("  ✅ Backend 显式指定路由目标")
	fmt.Println("  ✅ 支持灵活的路由策略（UserID 或 SessionID）")
	fmt.Println("  ✅ 向后兼容（新字段可选）")
	fmt.Println()
	fmt.Println("📝 使用方式:")
	fmt.Println("  Chat Service 在构建响应时设置 TargetUserId")
	fmt.Println("  Gateway 读取该字段决定转发目标")
	fmt.Println()
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("演示完成！✓")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
