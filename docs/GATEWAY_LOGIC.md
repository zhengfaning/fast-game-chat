# Gateway 逻辑说明文档

## 概述

Gateway 是整个系统的**连接管理层**和**消息分发中心**，负责：
1. 管理客户端 WebSocket 连接
2. 解析二进制协议头并路由消息
3. 通过 Redis MQ 与后端服务异步通信
4. 将后端响应推送给对应的客户端

**设计原则**: Gateway 是**游戏无关**的，只解析通用协议头，不理解业务逻辑。

---

## 核心组件

### 1. Server (连接管理器)
**位置**: `game-gateway/internal/server/server.go`

**职责**:
- 监听 HTTP 端口（默认 8080）
- 将 HTTP 升级为 WebSocket 连接
- 为每个连接创建 Session
- 启动读写循环（readPump / writePump）

**关键配置**:
```go
ReadBufferSize:  8192  // 8KB 读缓冲
WriteBufferSize: 8192  // 8KB 写缓冲
Send Channel:    1024  // 每个 Session 的发送队列
ReadLimit:       16MB  // 单个消息最大 16MB
```

---

### 2. Session (会话管理)
**位置**: `game-gateway/internal/session/manager.go`

**Session 生命周期**:
```
1. 客户端连接 → 创建 Session (分配 UUID)
2. 客户端发送消息 → 自动绑定 UserID (首次)
3. 后端响应到达 → 根据 UserID 查找 Session
4. 客户端断开 → 清理 Session
```

**Session 结构**:
```go
type Session struct {
    ID        string           // UUID (唯一标识)
    UserID    int32            // 用户ID (首次消息时绑定)
    Conn      *websocket.Conn  // WebSocket 连接
    Send      chan []byte      // 发送队列 (1024 缓冲)
    AuthToken string           // 认证令牌
}
```

**并发安全**: SessionManager 使用 `sync.RWMutex` 保护内部 map。

---

### 3. Router (消息路由器)
**位置**: `game-gateway/internal/router/router.go`

**路由决策流程**:
```
收到消息 → 解析协议头 (8字节) → 判断 Route 字段
    ├─ RouteChat (0x01)   → 发布到 Redis "game:request:{game_id}"
    ├─ RouteGame (0x02)   → (未实现，预留)
    └─ RouteSystem (0x03) → 心跳等系统消息，直接返回
```

**关键逻辑**:
1. **自动绑定 UserID**: 首次收到消息时，从 Protobuf 中提取 `user_id` 并绑定到 Session
2. **MQ 发布**: 将消息 Payload 原封不动地发布到 Redis Topic
3. **零业务耦合**: 不解析业务字段，只读取 `game_id` 用于 Topic 路由

---

## 消息流转详解

### Upstream (客户端 → 后端)

```
┌─────────┐    WebSocket     ┌─────────┐    解析协议头    ┌────────┐
│ 客户端  │ ──────────────→ │ Server  │ ──────────────→ │ Router │
└─────────┘   Binary Packet  └─────────┘   Packet Object  └────────┘
                                                              │
                                                              │ 提取 game_id
                                                              ↓
                                                          ┌────────┐
                                                          │ Redis  │
                                                          │  MQ    │
                                                          └────────┘
                                                              │
                                                              │ Pub: game:request:mmo
                                                              ↓
                                                          ┌────────┐
                                                          │  Chat  │
                                                          │Service │
                                                          └────────┘
```

**步骤详解**:
1. **readPump** 从 WebSocket 读取二进制数据
2. **protocol.ReadPacket** 解析协议头（Seq, Route, Payload）
3. **Router.RoutePacket** 根据 Route 分发
4. **Router.routeChatPacket** 解析 Payload 获取 `game_id`
5. **mqProducer.Publish** 发布到 `game:request:{game_id}`

---

### Downstream (后端 → 客户端)

```
┌────────┐    Pub: broadcast    ┌────────┐    Subscribe    ┌────────┐
│  Chat  │ ───────────────────→ │ Redis  │ ──────────────→ │ Router │
│Service │                       │  MQ    │                 └────────┘
└────────┘                       └────────┘                     │
                                                                │ 解析 TargetUserId
                                                                ↓
                                                            ┌────────┐
                                                            │Session │
                                                            │Manager │
                                                            └────────┘
                                                                │ 查找 Session
                                                                ↓
                                                            ┌─────────┐
                                                            │writePump│
                                                            └─────────┘
                                                                │
                                                                ↓
                                                            ┌─────────┐
                                                            │ 客户端  │
                                                            └─────────┘
```

**步骤详解**:
1. **main.go** 订阅 Redis `broadcast` 频道
2. **Router.HandleBroadcast** 接收消息
3. 尝试解析为 `ChatResponse` 或 `MessageBroadcast`
4. 根据 `TargetUserId` 或 `TargetSessionId` 查找 Session
5. 将消息推入 `Session.Send` 队列
6. **writePump** 从队列取出并发送给客户端

---

## 异常处理机制

### 1. 连接断开
**触发条件**:
- 客户端主动关闭
- 网络超时
- 读取错误

**处理流程**:
```go
defer func() {
    sess.Conn.Close()
    s.sessions.Remove(sess.ID)
    metrics.GlobalMetrics.IncrementDisconnections()
    log.Printf("[DISCONN] Session: %s", sess.ID)
}()
```

### 2. 消息解析失败
**场景**: 协议头损坏或 Protobuf 解析失败

**处理**: 记录错误日志，继续处理下一条消息（不中断连接）

### 3. 路由目标不存在
**场景**: 收到 `broadcast` 消息，但目标用户不在本 Gateway

**处理**: 
```go
if targetSession == nil {
    logger.Warn(logger.TagRouter, "Target not found | UserID: %d", targetUserID)
    return // 忽略，可能在其他 Gateway 实例
}
```

### 4. 发送队列满
**场景**: `Session.Send` 队列达到 1024 上限

**处理**:
```go
select {
case sess.Send <- message:
    // 成功发送
default:
    log.Printf("[QUEUE-WARN] Send buffer full | Session: %s", sess.ID)
    // 丢弃消息或关闭连接（当前实现是丢弃）
}
```

### 5. Panic 恢复
**位置**: readPump 和 writePump 都有 `defer recover()`

**处理**:
```go
defer func() {
    if r := recover(); r != nil {
        log.Printf("[PANIC] Recovered | Session: %s | Error: %v", sess.ID, r)
    }
}()
```

---

## 性能优化要点

### 1. 非阻塞设计
- **MQ Publish**: 异步发布，不等待后端响应
- **Send Channel**: 使用带缓冲的 channel，避免 writePump 阻塞

### 2. 并发处理
- 每个 Session 独立的 readPump 和 writePump goroutine
- SessionManager 使用读写锁，读操作并发

### 3. 内存优化
- Session 断开后立即清理
- 使用对象池复用 Packet 对象（可选优化）

### 4. 监控指标
```go
metrics.GlobalMetrics.IncrementConnections()      // 连接数
metrics.GlobalMetrics.IncrementMessagesReceived() // 接收消息数
metrics.GlobalMetrics.IncrementMessagesSent()     // 发送消息数
metrics.GlobalMetrics.IncrementRoutingErrors()    // 路由错误数
```

---

## 常见问题处理

### Q1: 客户端收不到消息
**排查步骤**:
1. 检查 Session 是否已绑定 UserID（查看日志 `[Binding Session]`）
2. 确认后端响应中的 `TargetUserId` 是否正确
3. 检查 `Session.Send` 队列是否满（日志 `[QUEUE-WARN]`）
4. 确认 writePump 是否正常运行

### Q2: 消息发送失败
**可能原因**:
- Redis 连接断开 → 检查 Redis 服务状态
- Topic 名称错误 → 确认 `game_id` 是否正确
- Payload 过大 → 检查是否超过 16MB 限制

### Q3: 内存泄漏
**排查**:
- 检查 Session 是否正常清理（断开时是否调用 `Remove`）
- 检查 Send channel 是否被正确关闭
- 使用 `pprof` 分析 goroutine 泄漏

### Q4: 高并发下性能下降
**优化方向**:
1. 增加 `Send` channel 缓冲大小
2. 调整 `ReadBufferSize` 和 `WriteBufferSize`
3. 使用多个 Gateway 实例水平扩展
4. 优化 SessionManager 锁粒度（考虑分片）

---

## 配置参数说明

**gateway.yaml**:
```yaml
server:
  host: "0.0.0.0"
  port: 8080

redis:
  addr: "localhost:6379"
  password: ""

games:
  - id: "mmo"
    chat_backend:
      host: "localhost"
      port: 9002
```

**关键参数**:
- `server.port`: Gateway 监听端口
- `redis.addr`: Redis 地址（用于 MQ）
- `games[].id`: 游戏标识（用于 Topic 路由）

---

## 总结

Gateway 的核心职责是**连接管理**和**消息分发**，通过以下设计实现高性能：
1. **协议驱动**: 只解析协议头，业务无关
2. **异步通信**: 通过 Redis MQ 解耦前后端
3. **并发处理**: 每个连接独立 goroutine
4. **优雅降级**: 完善的异常处理和恢复机制

在实际运维中，重点关注：
- Session 生命周期管理
- 消息队列健康状态
- 性能指标监控
- 异常日志分析
