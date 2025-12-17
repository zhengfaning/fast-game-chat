package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"game-gateway/pkg/protocol"
	"game-protocols/chat"
	"game-protocols/common"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

func main() {
	log.Println("========== WebSocket 二进制协议演示 ==========\n")

	// 连接到 Gateway
	u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
	log.Printf("连接到: %s\n", u.String())

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("连接失败:", err)
	}
	defer conn.Close()

	// 创建协议包装器
	wsConn := protocol.NewWSConn(conn)
	log.Println("✅ 已连接\n")

	// ========== 示例 1: 发送聊天消息 ==========
	log.Println("【示例 1】发送聊天消息")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━")

	// 1. 创建业务消息 (ChatRequest)
	chatReq := &chat.ChatRequest{
		Base: &common.MessageBase{
			GameId:    "mmo",
			UserId:    1001,
			Timestamp: time.Now().Unix(),
		},
		ReceiverId: 1002,
		Content:    "Hello from binary protocol!",
		Type:       chat.ChatRequest_TEXT,
	}

	// 2. 序列化业务消息
	payload, err := proto.Marshal(chatReq)
	if err != nil {
		log.Fatal("序列化失败:", err)
	}
	log.Printf("业务消息大小: %d bytes\n", len(payload))

	// 3. 发送请求（自动生成序列号）
	seq, err := wsConn.SendRequest(protocol.RouteChat, payload)
	if err != nil {
		log.Fatal("发送失败:", err)
	}
	log.Printf("✅ 已发送 (序列号: %d)\n", seq)

	// 4. 等待响应
	log.Println("\n等待服务器响应...")
	timeout := time.After(5 * time.Second)
	
	select {
	case <-timeout:
		log.Println("⏱ 超时")
	default:
		wsConn.SetReadLimit(1024 * 1024) // 1MB 限制
		pkt, err := wsConn.ReadPacket()
		if err != nil {
			log.Printf("读取响应失败: %v\n", err)
		} else {
			log.Printf("📨 收到响应: %s\n", pkt)
			
			// 检查序列号是否匹配
			if pkt.Sequence == seq {
				log.Println("✅ 序列号匹配")
			}
			
			// 解析响应
			var resp chat.ChatResponse
			if err := proto.Unmarshal(pkt.Payload, &resp); err == nil {
				log.Printf("ChatResponse: Success=%v, MsgID=%d\n", resp.Success, resp.MessageId)
			}
		}
	}

	// ========== 示例 2: 协议头详解 ==========
	log.Println("\n【示例 2】协议头结构分析")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━")

	// 创建一个测试数据包
	testPayload := []byte("test data")
	testPkt := protocol.NewPacketWithSeq(protocol.RouteChat, 12345, testPayload)
	testPkt.Flags.SetFlag(protocol.FlagCompressed) // 设置压缩标志
	
	encoded := testPkt.Encode()
	
	log.Printf("完整数据包: %d bytes\n", len(encoded))
	log.Printf("头部大小: %d bytes\n", protocol.HeaderSize)
	log.Printf("Payload: %d bytes\n", len(testPayload))

	// 解析头部
	route, flags, payloadLen, seq2, err := protocol.DecodeHeader(encoded)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("\n解析结果:\n")
	log.Printf("  Route: %d (%s)\n", route, getRouteName(route))
	log.Printf("  Flags: 0x%02X ", flags)
	if flags.HasFlag(protocol.FlagCompressed) {
		log.Printf("(压缩) ")
	}
	if flags.HasFlag(protocol.FlagEncrypted) {
		log.Printf("(加密) ")
	}
	log.Println()
	log.Printf("  Payload Length: %d bytes\n", payloadLen)
	log.Printf("  Sequence: %d\n", seq2)

	// 显示十六进制
	log.Printf("\n十六进制头部:\n  ")
	for i := 0; i < protocol.HeaderSize && i < len(encoded); i++ {
		log.Printf("%02X ", encoded[i])
		if i == 3 || i == 4 || i == 5 || i == 7 || i == 11 {
			log.Printf("| ")
		}
	}
	log.Println()
	log.Println("  ↑Magic    ↑R ↑F ↑Rsv  ↑Length    ↑Sequence")

	// ========== 示例 3: 性能对比 ==========
	log.Println("\n【示例 3】协议开销分析")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━")

	sizes := []int{50, 100, 500, 1000, 5000}
	log.Printf("%-15s %-15s %-15s %-10s\n", "业务数据", "完整包大小", "开销", "开销比例")
	log.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	
	for _, size := range sizes {
		testData := make([]byte, size)
		testPkt := protocol.NewPacket(protocol.RouteChat, testData)
		encoded := testPkt.Encode()
		overhead := len(encoded) - size
		ratio := float64(overhead) / float64(size) * 100
		
		log.Printf("%-15d %-15d %-15d %.2f%%\n", size, len(encoded), overhead, ratio)
	}

	log.Println("\n========== 演示结束 ==========")
}

func getRouteName(route protocol.RouteType) string {
	switch route {
	case protocol.RouteGame:
		return "GAME"
	case protocol.RouteChat:
		return "CHAT"
	case protocol.RouteSystem:
		return "SYSTEM"
	default:
		return "UNKNOWN"
	}
}
