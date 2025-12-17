package main

import (
	"fmt"
	"log"
	"time"

	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"
	"google.golang.org/protobuf/proto"
)

func main() {
	log.Println("========== 增强版二进制协议详解 ==========\n")

	// ========== 1. 协议头结构 ==========
	log.Println("【1】协议头结构 (16 bytes)")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("+-------+-------+-------+----------+--------+----------+")
	log.Println("| Magic | Route | Flags | Reserved | Length | Sequence |")
	log.Println("|(4byte)|(1byte)|(1byte)| (2 byte) |(4 byte)| (4 byte) |")
	log.Println("+-------+-------+-------+----------+--------+----------+")
	log.Println()
	log.Printf("总大小: %d bytes (固定)\n", protocol.HeaderSize)
	log.Printf("Magic Number: 0x%08X\n", protocol.MagicNumber)
	log.Println()

	// ========== 2. 字段说明 ==========
	log.Println("【2】字段详解")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("Magic (4 bytes):")
	log.Println("  - 固定值: 0x12345678")
	log.Println("  - 用途: 快速验证数据包有效性")
	log.Println()
	log.Println("Route (1 byte):")
	log.Println("  - 1 = GAME   (游戏逻辑)")
	log.Println("  - 2 = CHAT   (聊天消息)")
	log.Println("  - 3 = SYSTEM (系统消息)")
	log.Println("  - 用途: Gateway 路由决策")
	log.Println()
	log.Println("Flags (1 byte):")
	log.Println("  - bit 0: 是否压缩")
	log.Println("  - bit 1: 是否加密")
	log.Println("  - bit 2-7: 保留")
	log.Println("  - 用途: 未来功能扩展")
	log.Println()
	log.Println("Reserved (2 bytes):")
	log.Println("  - 当前未使用")
	log.Println("  - 用途: 未来协议升级")
	log.Println()
	log.Println("Length (4 bytes):")
	log.Println("  - Payload 长度（不含头部）")
	log.Println("  - 最大: 16MB")
	log.Println()
	log.Println("Sequence (4 bytes):")
	log.Println("  - 请求-响应匹配")
	log.Println("  - 客户端递增生成")
	log.Println()

	// ========== 3. 实际示例 ==========
	log.Println("【3】实际数据包示例")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// 创建业务消息
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

	// 序列化
	payload, _ := proto.Marshal(chatReq)
	log.Printf("业务消息 (ChatRequest) 大小: %d bytes\n", len(payload))

	// 创建数据包
	pkt := protocol.NewPacketWithSeq(protocol.RouteChat, 12345, payload)
	pkt.Flags.SetFlag(protocol.FlagCompressed)

	// 编码
	encoded := pkt.Encode()
	log.Printf("完整数据包大小: %d bytes\n", len(encoded))
	log.Printf("  - 头部: %d bytes (%.1f%%)\n", protocol.HeaderSize, 
		float64(protocol.HeaderSize)/float64(len(encoded))*100)
	log.Printf("  - Payload: %d bytes (%.1f%%)\n", len(payload),
		float64(len(payload))/float64(len(encoded))*100)

	// 显示十六进制
	log.Println("\n完整数据包 (十六进制):")
	log.Print("  头部: ")
	for i := 0; i < protocol.HeaderSize; i++ {
		fmt.Printf("%02X ", encoded[i])
		if i == 3 || i == 4 || i == 5 || i == 7 || i == 11 {
			fmt.Print("│ ")
		}
	}
	log.Println()
	log.Println("        ↑Magic      ↑R ↑F ↑Rsv    ↑Length      ↑Sequence")
	
	log.Print("  Payload: ")
	if len(payload) > 20 {
		for i := 0; i < 20; i++ {
			fmt.Printf("%02X ", payload[i])
		}
		fmt.Printf("... (%d more bytes)", len(payload)-20)
	} else {
		for _, b := range payload {
			fmt.Printf("%02X ", b)
		}
	}
	log.Println()

	// 解析验证
	log.Println("\n解析验证:")
	route, flags, length, seq, err := protocol.DecodeHeader(encoded)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("  Route: %d (CHAT)\n", route)
	log.Printf("  Flags: 0x%02X (", flags)
	if flags.HasFlag(protocol.FlagCompressed) {
		fmt.Print("Compressed ")
	}
	if flags.HasFlag(protocol.FlagEncrypted) {
		fmt.Print("Encrypted ")
	}
	fmt.Println(")")
	log.Printf("  Length: %d bytes\n", length)
	log.Printf("  Sequence: %d\n", seq)

	// ========== 4. Gateway 处理流程 ==========
	log.Println("\n【4】Gateway 处理流程")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("1. 从 WebSocket 读取二进制消息")
	log.Printf("   → 收到 %d bytes\n", len(encoded))
	log.Println()
	log.Println("2. 读取头部 (仅前 16 bytes)")
	log.Println("   route, flags, length, seq := DecodeHeader(data[:16])")
	log.Printf("   → Route=%d, Flags=0x%02X, Length=%d, Seq=%d\n", route, flags, length, seq)
	log.Println()
	log.Println("3. 根据 Route 决定转发目标")
	log.Println("   switch route {")
	log.Println("   case RouteChat:")
	log.Println("       → 转发到 Chat Service")
	log.Println("   }")
	log.Println()
	log.Println("4. 提取 Payload 并转发")
	log.Printf("   payload := data[16:]  // %d bytes\n", len(payload))
	log.Println("   chatService.Send(payload)  // 直接转发 Protobuf！")
	log.Println()
	log.Println("✅ Gateway 无需解析/编码 Protobuf！")

	// ========== 5. 性能优势 ==========
	log.Println("\n【5】性能优势分析")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	// 模拟性能测试
	iterations := 100000
	
	// 只测试头部解析
	start2 := time.Now()
	for i := 0; i < iterations; i++ {
		protocol.DecodeHeader(encoded)
	}
	elapsed2 := time.Since(start2)
	
	log.Printf("处理 %d 次:\n", iterations)
	log.Printf("  只读头部: %v (%.2f ns/次)\n", elapsed2,
		float64(elapsed2.Nanoseconds())/float64(iterations))
	log.Printf("\n说明: 头部解析极快，约 %.0f ns/次\n", 
		float64(elapsed2.Nanoseconds())/float64(iterations))

	// ========== 6. 开销对比 ==========
	log.Println("\n【6】协议开销对比表")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("%-15s %-15s %-10s %-10s\n", "Payload 大小", "总大小", "开销", "开销比")
	log.Println("─────────────────────────────────────────────────")
	
	sizes := []int{50, 100, 200, 500, 1000, 2000, 5000, 10000}
	for _, size := range sizes {
		testData := make([]byte, size)
		testPkt := protocol.NewPacket(protocol.RouteChat, testData)
		encoded := testPkt.Encode()
		overhead := len(encoded) - size
		ratio := float64(overhead) / float64(size) * 100
		
		log.Printf("%-15d %-15d %-10d %.2f%%\n", size, len(encoded), overhead, ratio)
	}

	log.Println("\n结论: 对于大多数消息 (>100 bytes)，开销 < 16%")
	log.Println("     对于大消息 (>1KB)，开销 < 2%")

	// ========== 7. 使用建议 ==========
	log.Println("\n【7】使用建议")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Println("✅ 适用场景:")
	log.Println("  - WebSocket 客户端 ↔ Gateway")
	log.Println("  - 需要快速路由的场景")
	log.Println("  - 支持请求-响应模式")
	log.Println("  - 未来需要压缩/加密")
	log.Println()
	log.Println("✅ Gateway → Backend:")
	log.Println("  - 只转发 Payload (纯 Protobuf)")
	log.Println("  - 无需协议头（内部稳定连接）")
	log.Println()
	log.Println("✅ 扩展性:")
	log.Println("  - Flags 支持压缩/加密标志")
	log.Println("  - Reserved 字段预留扩展空间")
	log.Println("  - Sequence 支持异步响应匹配")

	log.Println("\n========== 演示结束 ==========")
}
