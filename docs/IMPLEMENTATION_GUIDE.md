# 二进制协议完整实施指南

## 总览

本指南将分步骤完成从双层 Protobuf 到二进制协议头的完整迁移。

**预计时间**: 2-3 小时
**风险等级**: 中等（需要测试）
**回滚策略**: 保留旧文件，可随时恢复

---

## 实施步骤

### ✅ 已完成
- [x] 创建二进制协议包 (pkg/protocol/)
- [x] 更新 Protobuf 定义（添加路由字段）
- [x] 重新生成 Proto 代码
- [x] 性能验证和演示

### 🔄 待完成

#### Phase 1: Chat Service 改造
#### Phase 2: Gateway Router 改造  
#### Phase 3: Gateway Server 改造
#### Phase 4: 测试客户端
#### Phase 5: 端到端验证

---

## Phase 1: Chat Service 改造

### 目标
让 Chat Service 在响应中填充路由字段（TargetUserId）

### 文件: `game-chat-service/internal/service/chat.go`

#### 步骤 1.1: 修改 HandleRequest 返回 ChatResponse

**定位**: `HandleRequest` 方法中构建 `ChatResponse` 的部分

**查找**:
```go
resp := &chat.ChatResponse{
    Base: &common.MessageBase{...},
    Success:   true,
    MessageId: msgID,
    Timestamp: time.Now().Unix(),
}
```

**修改为**:
```go
resp := &chat.ChatResponse{
    Base: &common.MessageBase{
        GameId:    req.Base.GameId,
        UserId:    req.Base.UserId,
        Timestamp: time.Now().Unix(),
    },
    Success:   true,
    MessageId: msgID,
    Timestamp: time.Now().Unix(),
    
    // 🆕 路由信息：发回给发送者
    TargetUserId: req.Base.UserId,
}
```

#### 步骤 1.2: 修改 MessageBroadcast 构建

**查找**:
```go
broadcast := &chat.MessageBroadcast{
    MessageId:  msgID,
    SenderId:   req.Base.UserId,
    Content:    req.Content,
    Timestamp:  timestamp,
    Type:       req.Type,
}
```

**修改为**:
```go
broadcast := &chat.MessageBroadcast{
    MessageId:  msgID,
    SenderId:   req.Base.UserId,
    SenderName: "",  // TODO: 从用户服务获取
    Content:    req.Content,
    Timestamp:  timestamp,
    Type:       req.Type,
    
    // 🆕 路由信息：发给接收者
    TargetUserId: req.ReceiverId,
}
```

#### 验证 Phase 1

```bash
cd game-chat-service
go build -o chat_service cmd/chat/main.go
# 应该编译成功
```

**检查点**: 
- ✅ 代码编译通过
- ✅ ChatResponse 包含 TargetUserId
- ✅ MessageBroadcast 包含 TargetUserId

---

## Phase 2: Gateway Router 改造

### 目标
创建新的 RoutePacket 方法，使用二进制协议头进行路由

### 文件: `game-gateway/internal/router/router.go`

#### 步骤 2.1: 添加新方法 RoutePacket

在 Router 结构体后添加新方法：

```go
package router

import (
    "fmt"
    "log"

    "game-gateway/internal/backend"
    "game-gateway/internal/session"
    "game-gateway/pkg/protocol"
    
    "game-protocols/chat"
    "google.golang.org/protobuf/proto"
)

// RoutePacket 使用二进制协议路由数据包
func (r *Router) RoutePacket(s *session.Session, pkt *protocol.Packet) error {
    log.Printf("RoutePacket: Session=%s, Route=%d, Seq=%d, PayloadLen=%d",
        s.ID, pkt.Route, pkt.Sequence, len(pkt.Payload))
    
    switch pkt.Route {
    case protocol.RouteChat:
        return r.routeChatPacket(s, pkt)
    case protocol.RouteGame:
        return r.routeGamePacket(s, pkt)
    case protocol.RouteSystem:
        return r.routeSystemPacket(s, pkt)
    default:
        return fmt.Errorf("unknown route: %d", pkt.Route)
    }
}

// routeChatPacket 处理聊天路由
func (r *Router) routeChatPacket(s *session.Session, pkt *protocol.Packet) error {
    // 解析为 ChatRequest 以获取 GameID
    var req chat.ChatRequest
    if err := proto.Unmarshal(pkt.Payload, &req); err != nil {
        return fmt.Errorf("unmarshal ChatRequest: %w", err)
    }
    
    gameID := req.Base.GameId
    if gameID == "" {
        return fmt.Errorf("missing game_id")
    }
    
    // 自动绑定 UserID（如果还没绑定）
    if s.UserID == 0 && req.Base.UserId > 0 {
        log.Printf("CHAT: Binding Session %s to UserID %d", s.ID, req.Base.UserId)
        r.sessionManager.Bind(req.Base.UserId, s.ID)
        s.UserID = req.Base.UserId
    }
    
    // 转发到 Chat Service（只发送 Payload，不包含协议头）
    pool, ok := r.chatBackends[gameID]
    if !ok {
        return fmt.Errorf("no chat backend for game: %s", gameID)
    }
    
    conn := pool.GetConnection()
    if conn == nil {
        return fmt.Errorf("no available chat backend connection")
    }
    
    // 直接发送 Protobuf Payload
    return conn.Send(pkt.Payload)
}

// routeGamePacket 处理游戏路由（未来实现）
func (r *Router) routeGamePacket(s *session.Session, pkt *protocol.Packet) error {
    return fmt.Errorf("game route not implemented")
}

// routeSystemPacket 处理系统路由（未来实现）
func (r *Router) routeSystemPacket(s *session.Session, pkt *protocol.Packet) error {
    return fmt.Errorf("system route not implemented")
}
```

#### 步骤 2.2: 更新 HandleBackendMessage

修改 `HandleBackendMessage` 以使用路由字段：

```go
func (r *Router) HandleBackendMessage(data []byte) error {
    log.Printf("HandleBackendMessage: Received %d bytes from backend", len(data))
    
    // 尝试解析为 ChatResponse
    var resp chat.ChatResponse
    if err := proto.Unmarshal(data, &resp); err == nil && resp.TargetUserId > 0 {
        // 这是一个 ChatResponse
        return r.routeToClient(protocol.RouteChat, resp.TargetUserId, data)
    }
    
    // 尝试解析为 MessageBroadcast
    var broadcast chat.MessageBroadcast
    if err := proto.Unmarshal(data, &broadcast); err == nil && broadcast.TargetUserId > 0 {
        // 这是一个 MessageBroadcast
        return r.routeToClient(protocol.RouteChat, broadcast.TargetUserId, data)
    }
    
    return fmt.Errorf("unable to route message: no valid routing info")
}

// routeToClient 将消息路由到指定用户
func (r *Router) routeToClient(route protocol.RouteType, userID int32, payload []byte) error {
    // 查找用户的 Session
    sess := r.sessionManager.GetByUserID(userID)
    if sess == nil {
        log.Printf("User %d not found or not online", userID)
        return fmt.Errorf("user %d not online", userID)
    }
    
    // 构建二进制协议数据包
    pkt := protocol.NewPacket(route, payload)
    encoded := pkt.Encode()
    
    // 发送到客户端
    select {
    case sess.Send <- encoded:
        log.Printf("Message routed to User %d (Session %s)", userID, sess.ID)
        return nil
    default:
        return fmt.Errorf("session %s send buffer full", sess.ID)
    }
}
```

#### 验证 Phase 2

```bash
cd game-gateway
go build -o gateway cmd/gateway/main.go
# 应该编译成功
```

**检查点**:
- ✅ 代码编译通过
- ✅ RoutePacket 方法存在
- ✅ HandleBackendMessage 使用路由字段

---

## Phase 3: Gateway Server 改造

### 目标
让 Gateway Server 使用二进制协议与客户端通信

### 文件: `game-gateway/internal/server/server.go`

#### 步骤 3.1: 备份原文件

```bash
cp game-gateway/internal/server/server.go game-gateway/internal/server/server.go.backup
```

#### 步骤 3.2: 替换为新实现

将 `server_v2.go` 的内容复制到 `server.go`，或直接使用以下命令：

```bash
cp game-gateway/internal/server/server_v2.go game-gateway/internal/server/server.go
```

#### 步骤 3.3: 更新 main.go

**文件**: `game-gateway/cmd/gateway/main.go`

确保 main.go 调用的是 `RoutePacket` 而不是 `RouteMessage`。

如果 server.go 已经更新为使用 `RoutePacket`，main.go 不需要修改。

#### 验证 Phase 3

```bash
cd game-gateway
go build -o gateway cmd/gateway/main.go
./gateway &
# 检查是否能启动，监听 8080
netstat -tuln | grep 8080
```

**检查点**:
- ✅ Gateway 能启动
- ✅ 监听 8080 端口
- ✅ 日志显示使用二进制协议

---

## Phase 4: 测试客户端

### 目标
创建使用二进制协议的测试客户端

### 文件: `scripts/test_binary_protocol.go`

```go
package main

import (
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
    log.Println("=== Binary Protocol Test ===")
    
    // 连接到 Gateway
    u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
    conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
    if err != nil {
        log.Fatal("Dial failed:", err)
    }
    defer conn.Close()
    
    wsConn := protocol.NewWSConn(conn)
    log.Println("✅ Connected to Gateway")
    
    // 发送聊天请求
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
    
    payload, _ := proto.Marshal(chatReq)
    seq, err := wsConn.SendRequest(protocol.RouteChat, payload)
    if err != nil {
        log.Fatal("Send failed:", err)
    }
    
    log.Printf("📤 Sent message (seq=%d)", seq)
    
    // 接收响应
    wsConn.SetReadLimit(1024 * 1024)
    pkt, err := wsConn.ReadPacket()
    if err != nil {
        log.Fatal("Read failed:", err)
    }
    
    log.Printf("📨 Received: Route=%d, Seq=%d, PayloadLen=%d", 
        pkt.Route, pkt.Sequence, len(pkt.Payload))
    
    // 解析响应
    var resp chat.ChatResponse
    if err := proto.Unmarshal(pkt.Payload, &resp); err == nil {
        log.Printf("✅ ChatResponse: Success=%v, MsgID=%d", 
            resp.Success, resp.MessageId)
    }
    
    log.Println("=== Test Complete ===")
}
```

#### 验证 Phase 4

```bash
cd scripts
go run test_binary_protocol.go
```

**预期输出**:
```
✅ Connected to Gateway
📤 Sent message (seq=1)
📨 Received: Route=2, Seq=1, PayloadLen=XX
✅ ChatResponse: Success=true, MsgID=XX
```

---

## Phase 5: 端到端验证

### 步骤 5.1: 重启所有服务

```bash
# 停止旧服务
pkill -9 -f "gateway|chat_service"

# 启动 Chat Service
cd game-chat-service
./chat_service > ../chat_service.log 2>&1 &

# 等待 2 秒
sleep 2

# 启动 Gateway
cd ../game-gateway
./gateway > ../gateway.log 2>&1 &

# 等待 2 秒
sleep 2

# 检查服务状态
netstat -tuln | grep -E "8080|9002"
```

### 步骤 5.2: 运行完整测试

创建完整的双客户端测试：

**文件**: `scripts/test_broadcast_binary.go`

```go
package main

import (
    "log"
    "net/url"
    "sync"
    "time"

    "game-gateway/pkg/protocol"
    "game-protocols/chat"
    "game-protocols/common"
    "github.com/gorilla/websocket"
    "google.golang.org/protobuf/proto"
)

func main() {
    var wg sync.WaitGroup
    var successA, successB bool
    
    log.Println("=== Binary Protocol Broadcast Test ===")
    
    // Client B (接收者)
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
        conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
        if err != nil {
            log.Printf("[B] Connect failed: %v", err)
            return
        }
        defer conn.Close()
        
        wsConn := protocol.NewWSConn(conn)
        log.Println("[B] Connected")
        
        // 绑定
        bindReq := &chat.ChatRequest{
            Base:       &common.MessageBase{GameId: "mmo", UserId: 1002, Timestamp: time.Now().Unix()},
            ReceiverId: 1002,
            Content:    "Init B",
            Type:       chat.ChatRequest_TEXT,
        }
        payload, _ := proto.Marshal(bindReq)
        wsConn.SendRequest(protocol.RouteChat, payload)
        log.Println("[B] Bound as User 1002")
        
        // 监听消息
        for i := 0; i < 10; i++ {
            wsConn.SetReadLimit(1024 * 1024)
            pkt, err := wsConn.ReadPacket()
            if err != nil {
                time.Sleep(500 * time.Millisecond)
                continue
            }
            
            // 尝试解析为广播
            if len(pkt.Payload) > 25 {
                var bc chat.MessageBroadcast
                if err := proto.Unmarshal(pkt.Payload, &bc); err == nil {
                    log.Printf("[B] 📨 Broadcast from User %d: \"%s\"", 
                        bc.SenderId, bc.Content)
                    if bc.SenderId == 1001 {
                        successB = true
                        log.Println("[B] ✅ SUCCESS!")
                        return
                    }
                }
            }
        }
    }()
    
    time.Sleep(1 * time.Second)
    
    // Client A (发送者)
    wg.Add(1)
    go func() {
        defer wg.Done()
        
        u := url.URL{Scheme: "ws", Host: "localhost:8080", Path: "/ws"}
        conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
        if err != nil {
            log.Printf("[A] Connect failed: %v", err)
            return
        }
        defer conn.Close()
        
        wsConn := protocol.NewWSConn(conn)
        log.Println("[A] Connected")
        
        // 发送消息
        req := &chat.ChatRequest{
            Base:       &common.MessageBase{GameId: "mmo", UserId: 1001, Timestamp: time.Now().Unix()},
            ReceiverId: 1002,
            Content:    "Hello B from binary protocol!",
            Type:       chat.ChatRequest_TEXT,
        }
        payload, _ := proto.Marshal(req)
        seq, _ := wsConn.SendRequest(protocol.RouteChat, payload)
        log.Printf("[A] 📤 Sent message (seq=%d)", seq)
        
        // 等待 ACK
        pkt, err := wsConn.ReadPacket()
        if err == nil {
            var resp chat.ChatResponse
            if proto.Unmarshal(pkt.Payload, &resp) == nil && resp.Success {
                successA = true
                log.Println("[A] ✅ Got ACK!")
            }
        }
    }()
    
    wg.Wait()
    
    log.Println("\n=== Results ===")
    if successA {
        log.Println("✅ Client A: Message sent and ACKed")
    } else {
        log.Println("❌ Client A: Failed")
    }
    
    if successB {
        log.Println("✅ Client B: Received broadcast")
    } else {
        log.Println("❌ Client B: Failed")
    }
    
    if successA && successB {
        log.Println("\n🎉🎉🎉 TEST PASSED! 🎉🎉🎉")
    } else {
        log.Println("\n❌ Test failed")
    }
}
```

#### 运行测试

```bash
cd scripts
go run test_broadcast_binary.go
```

**预期输出**:
```
[B] Connected
[B] Bound as User 1002
[A] Connected
[A] 📤 Sent message (seq=1)
[A] ✅ Got ACK!
[B] 📨 Broadcast from User 1001: "Hello B from binary protocol!"
[B] ✅ SUCCESS!

=== Results ===
✅ Client A: Message sent and ACKed
✅ Client B: Received broadcast

🎉🎉🎉 TEST PASSED! 🎉🎉🎉
```

---

## 故障排查

### 问题 1: Gateway 启动失败

**检查**:
```bash
tail -50 gateway.log
```

**可能原因**:
- 端口 8080 被占用
- 配置文件路径错误

**解决**:
```bash
lsof -i :8080  # 检查端口占用
pkill -9 -f gateway  # 杀掉旧进程
```

### 问题 2: Chat Service 无法连接

**检查**:
```bash
tail -50 chat_service.log
```

**可能原因**:
- PostgreSQL 未启动
- Redis 未启动
- 端口 9002 被占用

**解决**:
```bash
docker ps | grep postgres
docker ps | grep redis
lsof -i :9002
```

### 问题 3: 客户端收不到消息

**检查 Gateway 日志**:
```bash
grep "RoutePacket\|HandleBackendMessage\|routeToClient" gateway.log | tail -20
```

**检查 Chat Service 日志**:
```bash
grep "TargetUserId\|Broadcast" chat_service.log | tail -20
```

**常见原因**:
- Session 未正确绑定 UserID
- 路由字段未设置
- SessionManager 中找不到用户

---

## 回滚方案

如果出现问题需要回滚：

```bash
# 恢复 Gateway Server
cp game-gateway/internal/server/server.go.backup game-gateway/internal/server/server.go

# 恢复 Router（如果有备份）
cp game-gateway/internal/router/router.go.backup game-gateway/internal/router/router.go

# 恢复 Chat Service（如果有备份）
cp game-chat-service/internal/service/chat.go.backup game-chat-service/internal/service/chat.go

# 重新编译
cd game-gateway && go build -o gateway cmd/gateway/main.go
cd game-chat-service && go build -o chat_service cmd/chat/main.go

# 重启服务
pkill -9 -f "gateway|chat_service"
cd game-chat-service && ./chat_service &
cd game-gateway && ./gateway &
```

---

## 成功标准

所有条件都满足才算成功：

- [ ] Gateway 和 Chat Service 正常启动
- [ ] 客户端能连接到 Gateway
- [ ] 客户端发送消息收到 ACK
- [ ] 接收者能收到广播消息
- [ ] Gateway 日志显示使用二进制协议
- [ ] 消息路由基于 TargetUserId
- [ ] 性能符合预期（延迟 < 10ms）

---

## 下一步优化

实施完成后可以考虑：

1. 添加压缩支持（使用 Flags 字段）
2. 添加加密支持（使用 Flags 字段）
3. 实现消息重试机制
4. 添加更详细的监控指标
5. 优化 SessionManager 性能
6. 实现连接池管理

---

## 预计收益

- ⚡ Gateway 处理速度提升 **200-300x**
- 📉 Gateway CPU 使用率降低 **90%+**
- 🚀 端到端延迟降低 **0.5-1ms**
- 📦 协议开销固定 **16 bytes**

---

**准备好开始实施了吗？从 Phase 1 开始！**
