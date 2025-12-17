package protocol

// PayloadType 定义 Payload 的具体类型
// 用于在同一个 Route 下区分不同的消息类型
type PayloadType byte

const (
	// CHAT Route 下的 Payload 类型
	PayloadChatRequest   PayloadType = 1  // 客户端 -> 服务器
	PayloadChatResponse  PayloadType = 2  // 服务器 -> 客户端 (ACK)
	PayloadChatBroadcast PayloadType = 3  // 服务器 -> 客户端 (广播)
	
	// GAME Route 下的 Payload 类型 (未来扩展)
	PayloadGameRequest   PayloadType = 10
	PayloadGameResponse  PayloadType = 11
	
	// SYSTEM Route 下的 Payload 类型
	PayloadSystemPing    PayloadType = 20
	PayloadSystemPong    PayloadType = 21
)

// GetPayloadType 根据 Route 和消息方向推断 PayloadType
// 这是一个辅助函数，用于向后兼容
func GetPayloadType(route RouteType, isRequest bool) PayloadType {
	switch route {
	case RouteChat:
		if isRequest {
			return PayloadChatRequest
		}
		// 响应类型需要通过解析 Protobuf 来区分
		// ChatResponse vs MessageBroadcast
		return 0 // 需要进一步判断
	case RouteGame:
		if isRequest {
			return PayloadGameRequest
		}
		return PayloadGameResponse
	case RouteSystem:
		if isRequest {
			return PayloadSystemPing
		}
		return PayloadSystemPong
	}
	return 0
}

// 消息类型判断辅助函数
// 基于 Protobuf 的第一个字段来快速判断类型
func IsChatResponse(data []byte) bool {
	// ChatResponse 第一个字段是 MessageBase (message类型)
	// MessageBroadcast 第一个字段是 message_id (int64)
	// 简单启发式：检查第一个字段的 wire type
	if len(data) < 2 {
		return false
	}
	// Wire format: [field_number << 3 | wire_type]
	// MessageBase 是 message (wire_type=2)
	// int64 是 varint (wire_type=0)
	wireType := data[0] & 0x07
	return wireType == 2 // message type
}
