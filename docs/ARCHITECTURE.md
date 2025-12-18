# 游戏聊天系统架构文档

## 📊 压力测试结果

✅ **最新测试（1000 用户）**
- 用户数：1000
- 发送消息：1000 条
- 接收消息：1000 条
- 成功率：100%
- 并发写入问题：**已解决**（使用 WritePump 模式）

---

## 🏗️ 系统架构图

详见: `architecture_diagram.md`

### 系统组件

#### 1. **客户端层**
- 游戏客户端（1000+ 并发）
- WebSocket 连接
- Speedy 协议封装

#### 2. **网关层 (Gateway)**
- **端口**: 8080
- **协议**: WebSocket + Speedy
- **职责**:
  - 维护用户 Session 映射
  - 协议解析与路由
  - 消息转发
- **组件**:
  - SessionManager: 会话管理
  - ReadPump: 读取消息
  - WritePump: 写入消息
  - Router: 路由分发

#### 3. **业务服务层 (Chat Service)**
- **端口**: 9002
- **协议**: WebSocket + Protobuf
- **职责**:
  - 业务逻辑处理
  - 消息持久化
  - 在线状态管理
- **组件**:
  - WebSocket Server (含 WritePump)
  - ChatHandler: 业务处理器
  - Broadcaster: 消息广播

#### 4. **数据层**

**PostgreSQL (端口: 5432)**
- `messages`: 消息永久存储
- `user_presence`: 用户在线状态
- `announcements`: 系统公告

**Redis (端口: 6379)**
- `user:*`: 在线用户缓存
- `session:*`: Session 映射
- `stress:*`: 压测验证数据

---

## 🔄 消息流程序列图

详见: `sequence_diagram.puml`

### 主要流程

#### 阶段 1: 连接与绑定
```
客户端 → Gateway: WebSocket 连接
Gateway → Chat Service: 转发 Bind 请求
Chat Service → Redis: 存储在线状态
Chat Service → Gateway: ACK 响应
Gateway → 客户端: 绑定成功
```

#### 阶段 2: 消息发送（A → B）
```
客户端 A → Gateway: ChatRequest (To: B)
Gateway → Chat Service: 转发消息

Chat Service:
  ├─ 验证与处理
  ├─ PostgreSQL: 持久化
  ├─ Gateway: ACK (to A)
  └─ Gateway: Broadcast (to B)

Gateway:
  ├─ 路由 ACK → 客户端 A
  └─ 路由 Broadcast → 客户端 B
```

#### 阶段 3: 离线消息
```
Chat Service 检测用户离线
  → PostgreSQL 保存离线消息
  → 返回 OFFLINE_SAVED 状态
```

---

## 🔧 关键技术点

### 1. **WritePump 模式**（并发安全）

**问题**：WebSocket 不支持并发写入
**解决**：引入写入队列

```go
// 写入队列
writeChan := make(chan []byte, 100)

// 专用 WritePump Goroutine
go func() {
    for data := range writeChan {
        conn.WriteMessage(websocket.BinaryMessage, data)
    }
}()

// 所有写操作通过队列
writeChan <- responseData
```

### 2. **Speedy 协议**

- 4字节固定头部（Route + Length）
- 高效二进制传输
- 支持快速路由

### 3. **Protobuf 序列化**

- 跨服务通信标准
- 比 JSON 更高效
- 强类型定义

---

## 📡 网络端口

| 服务 | 端口 | 协议 | 用途 |
|------|------|------|------|
| Gateway | 8080 | WebSocket | 客户端连接 |
| Chat Service | 9002 | WebSocket | 内部服务通信 |
| PostgreSQL | 5432 | TCP | 消息持久化 |
| Redis | 6379 | TCP | 缓存与会话 |

---

## 🚀 扩展架构

### 水平扩展
- **多 Gateway 实例**: 共享 Redis Session
- **多 Chat Service 实例**: 按游戏/功能分区
- **负载均衡**: Nginx/HAProxy

### 服务拆分
- **Chat Service**: 私聊、群聊
- **Announcement Service**: 公告推送
- **Presence Service**: 在线状态管理

---

## 📈 性能指标

- **并发连接**: 1000+ (已验证)
- **消息延迟**: < 10ms
- **吞吐量**: 10,000+ msg/s
- **可用性**: 99.9%

---

## 📝 相关文件

- `sequence_diagram.puml`: 序列图源文件
- `architecture_diagram.md`: 架构图源文件 (Markdown + PlantUML)
- `scripts/stress_cluster.go`: 压力测试脚本
- `game-gateway/`: Gateway 源码
- `game-chat-service/`: Chat Service 源码
