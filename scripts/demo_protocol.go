package main

import (
	"fmt"
	"time"
	
	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"
	"game-protocols/gateway"
	"google.golang.org/protobuf/proto"
)

func main() {
	// 准备测试消息
	chatReq := &chat.ChatRequest{
		Base:       &common.MessageBase{GameId: "mmo", UserId: 1001, Timestamp: time.Now().Unix()},
		ReceiverId: 1002,
		Content:    "Hello World!",
		Type:       chat.ChatRequest_TEXT,
	}
	
	fmt.Println("========== 性能对比 ==========\n")
	
	// ==================== 方案1: 当前的 Protobuf Envelope ====================
	fmt.Println("【方案1】双层 Protobuf (当前实现)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Step 1: 序列化业务消息
	payload1, _ := proto.Marshal(chatReq)
	fmt.Printf("业务消息大小: %d bytes\n", len(payload1))
	
	// Step 2: 包装进 Envelope
	env := &gateway.Envelope{
		Route:   gateway.Envelope_CHAT,
		GameId:  "mmo",
		UserId:  1001,
		Payload: payload1,
	}
	
	// Step 3: 序列化 Envelope
	data1, _ := proto.Marshal(env)
	fmt.Printf("完整数据包大小: %d bytes\n", len(data1))
	fmt.Printf("开销: %d bytes (%.1f%%)\n", len(data1)-len(payload1), float64(len(data1)-len(payload1))/float64(len(payload1))*100)
	
	// Gateway 路由过程
	start1 := time.Now()
	for i := 0; i < 10000; i++ {
		var tmpEnv gateway.Envelope
		proto.Unmarshal(data1, &tmpEnv) // 完整解析
		// route := tmpEnv.Route
		// 重新编码转发
		proto.Marshal(&tmpEnv)
	}
	elapsed1 := time.Since(start1)
	fmt.Printf("Gateway 处理 10000 次: %v (%.2f μs/次)\n", elapsed1, float64(elapsed1.Microseconds())/10000.0)
	
	fmt.Println()
	
	// ==================== 方案2: 二进制头 + Protobuf ====================
	fmt.Println("【方案2】二进制头 + Protobuf (优化方案)")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// Step 1: 业务消息已序列化 (payload1)
	fmt.Printf("业务消息大小: %d bytes\n", len(payload1))
	
	// Step 2: 添加二进制头
	pkt := protocol.NewPacket(protocol.RouteChat, payload1)
	data2 := pkt.Encode()
	fmt.Printf("完整数据包大小: %d bytes\n", len(data2))
	fmt.Printf("开销: %d bytes (固定头部)\n", protocol.HeaderSize)
	
	// Gateway 路由过程
	start2 := time.Now()
	for i := 0; i < 10000; i++ {
		// 只读头部 9 字节
		route, payloadLen, _ := protocol.DecodeHeader(data2)
		_ = route
		_ = payloadLen
		// 直接转发 payload，无需重新编码
	}
	elapsed2 := time.Since(start2)
	fmt.Printf("Gateway 处理 10000 次: %v (%.2f μs/次)\n", elapsed2, float64(elapsed2.Microseconds())/10000.0)
	
	fmt.Println()
	
	// ==================== 对比总结 ====================
	fmt.Println("========== 对比总结 ==========")
	fmt.Printf("数据包大小: 方案1=%d bytes, 方案2=%d bytes, 节省=%d bytes\n", 
		len(data1), len(data2), len(data1)-len(data2))
	fmt.Printf("处理速度: 方案1=%.2fμs, 方案2=%.2fμs, 提升=%.1fx\n",
		float64(elapsed1.Microseconds())/10000.0,
		float64(elapsed2.Microseconds())/10000.0,
		float64(elapsed1)/float64(elapsed2))
	
	fmt.Println("\n========== 协议详情 ==========")
	fmt.Println("\n【方案1 - Protobuf Envelope】")
	fmt.Println("结构:")
	fmt.Println("  Client → Gateway: Protobuf(Envelope{Protobuf(ChatRequest)})")
	fmt.Println("  Gateway 处理: Unmarshal(Envelope) → 读 Route → Marshal(Envelope) → 转发")
	fmt.Println("  Gateway → Backend: Protobuf(Envelope{Protobuf(ChatRequest)})")
	fmt.Println("缺点:")
	fmt.Println("  ❌ Gateway 需要完整解析 Protobuf")
	fmt.Println("  ❌ Gateway 需要重新编码 Protobuf")
	fmt.Println("  ❌ 额外的 Protobuf 开销（动态长度编码）")
	
	fmt.Println("\n【方案2 - 二进制头】")
	fmt.Println("结构:")
	fmt.Println("  Client → Gateway: [Magic(4)][Route(1)][Len(4)][Protobuf(ChatRequest)]")
	fmt.Println("  Gateway 处理: 读取 9 字节头部 → 获得 Route")
	fmt.Println("  Gateway → Backend: [Protobuf(ChatRequest)] (直接转发 Payload)")
	fmt.Println("优点:")
	fmt.Println("  ✅ Gateway 只需读 9 字节固定头部")
	fmt.Println("  ✅ Gateway 无需解析/编码 Protobuf")
	fmt.Println("  ✅ 可以直接转发 Payload 二进制数据")
	fmt.Println("  ✅ 开销固定（9 字节）")
	fmt.Println("  ✅ 支持 Magic Number 校验")
	
	fmt.Println("\n========== 数据包示例 ==========")
	fmt.Println("\n方案2 数据包结构:")
	fmt.Printf("  Magic:   0x%08X (校验头)\n", protocol.MagicNumber)
	fmt.Printf("  Route:   %d (CHAT)\n", pkt.Route)
	fmt.Printf("  Length:  %d bytes\n", len(pkt.Payload))
	fmt.Printf("  Payload: [%d bytes Protobuf ChatRequest]\n", len(pkt.Payload))
	
	// 显示二进制头部
	header := data2[:9]
	fmt.Printf("\n  十六进制: ")
	for i, b := range header {
		fmt.Printf("%02X ", b)
		if i == 3 || i == 4 {
			fmt.Print("| ")
		}
	}
	fmt.Println()
	fmt.Println("              ↑ Magic   ↑Route ↑ Length")
}
