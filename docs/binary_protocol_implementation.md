# 二进制协议实现计划

## 目标
将系统从双层 Protobuf (Envelope{Payload}) 迁移到二进制头部 + 单层 Protobuf

## 架构变更

### 之前（双层 Protobuf）
```
Client → Gateway → Backend
  ↓         ↓          ↓
Envelope  解析      Envelope
{Payload} Envelope  {Payload}
```

问题：
- Gateway 需要完整解析/编码 Protobuf
- 性能开销大
- ChatResponse 和 MessageBroadcast 难以区分

### 之后（二进制头部）
```
Client → Gateway → Backend
  ↓         ↓          ↓
[Header+  读16字节   [Protobuf]
Protobuf] 快速路由   纯业务消息
```

优势：
- Gateway 只读 16 字节头部
- 直接转发 Protobuf Payload
- 性能提升 200+ 倍

## 实现步骤

### Phase 1: Gateway 改造 ✓
- [x] 创建 protocol 包
- [x] 实现二进制协议编解码
- [x] WebSocket 适配器
- [ ] 更新 Gateway Server
  - [ ] 使用 protocol.WSConn 替代原始 websocket.Conn
  - [ ] 更新 ReadPump 使用 ReadPacket
  - [ ] 更新 WritePump 使用 WritePacket
- [ ] 更新 Gateway Router
  - [ ] 读取 Route 字段路由
  - [ ] 转发纯 Protobuf 到 Backend

### Phase 2: Backend 改造
- [ ] Chat Service
  - [ ] 移除 Envelope 解析
  - [ ] 直接解析 ChatRequest
  - [ ] 返回纯 ChatResponse/MessageBroadcast
- [ ] Backend → Gateway 通信
  - [ ] 需要添加元信息（UserID/SessionID）
  - [ ] 方案1: 在 ChatResponse/MessageBroadcast 中添加路由字段
  - [ ] 方案2: Gateway 维护 Sequence → Session 映射

### Phase 3: 测试客户端
- [ ] 更新 verify_broadcast.go
  - [ ] 使用 protocol.WSConn
  - [ ] 发送时添加协议头
  - [ ] 接收时解析协议头
- [ ] 验证完整流程

### Phase 4: C# SDK
- [ ] 实现 C# 版本的协议编解码
- [ ] 更新 NetworkClient
- [ ] 更新 ChatManager

## 关键设计决策

### 1. Backend 返回消息如何路由？

**问题**: Backend (Chat Service) 返回 ChatResponse/MessageBroadcast，Gateway 如何知道发给谁？

**方案 A**: 修改 Protobuf 消息，添加路由字段
```protobuf
message ChatResponse {
    // 新增：路由信息
    string session_id = 10;  // 或 int32 user_id = 11;
    
    // 原有字段
    MessageBase base = 1;
    bool success = 2;
    ...
}
```

**方案 B**: Gateway 维护 Sequence 映射
```go
// 客户端发送时：seq=123, sessionID=abc
seqMap[123] = "abc"

// Backend 响应时：seq=123
sessionID := seqMap[123]  // 找到 "abc"
```

**推荐**: 方案 A - 在消息中添加路由字段
- 简单直接
- 无状态，易于扩展
- 支持广播（一条消息发给多个用户）

### 2. 消息类型如何区分？

现在 CHAT Route 下有两种响应：
- ChatResponse (ACK)
- MessageBroadcast (广播)

**方案**: Wire Type 判断（无需修改 Protobuf）
```go
// ChatResponse 第一个字段: base (MessageBase, wire_type=2)
// MessageBroadcast 第一个字段: message_id (int64, wire_type=0)
wireType := data[0] & 0x07
if wireType == 2 {
    // ChatResponse
} else {
    // MessageBroadcast
}
```

## 兼容性

### 渐进迁移策略
1. Gateway 同时支持两种协议（通过 Magic Number 检测）
2. 客户端逐步迁移
3. 完全迁移后移除旧协议

### Magic Number检测
```go
func DetectProtocol(data []byte) Protocol {
    if len(data) < 4 {
        return ProtocolUnknown
    }
    magic := binary.BigEndian.Uint32(data[0:4])
    if magic == 0x12345678 {
        return ProtocolBinary
    }
    // 尝试解析为 Protobuf Envelope
    var env gateway.Envelope
    if proto.Unmarshal(data, &env) == nil {
        return ProtocolProtobuf
    }
    return ProtocolUnknown
}
```

## 执行顺序

推荐按以下顺序实施，确保系统始终可用：

1. **Gateway Server 改造** - 支持新协议，保持向后兼容
2. **测试客户端** - 验证 Gateway 新协议
3. **Chat Service 改造** - 简化为纯 Protobuf
4. **端到端测试** - 验证完整流程
5. **C# SDK 更新** - 移植到生产客户端
6. **移除旧协议** - 清理兼容代码

## 性能预期

基于 demo_protocol.go 的测试结果：

- 数据包大小：减少 3-6 bytes (取决于 Protobuf 动态编码)
- Gateway 处理速度：提升 200-300 倍
- 延迟降低：约 0.5 μs/消息

对于高并发场景（10万 CCU）：
- 旧方案: 10万 × 0.59μs = 59ms CPU时间/批次
- 新方案: 10万 × 0.002μs = 0.2ms CPU时间/批次

CPU 节省：**99%+**
