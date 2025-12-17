# 网关统一连接 + 聊天系统实施计划

> **方案**: 协议头标识 + 单连接多路复用（bytes payload）
> 
> **核心思想**: 客户端只维护一条到网关的 WebSocket 连接，所有消息（游戏+聊天）通过 Envelope 封装，网关根据路由字段透明转发到对应后端服务。

---

## 架构概览

```
客户端 
   ↓ (单条 WebSocket 连接)
   ↓ 
网关 (Go Gateway)
   ├─→ C++ 游戏逻辑服 (GLS) [路由: GAME]
   └─→ Go 聊天服务 (GCS)    [路由: CHAT]
```

---

## 阶段划分

### 📋 总体时间线

| 阶段 | 预估时间 | 关键交付物 |
|------|----------|-----------|
| **阶段 0**: 协议设计与定义 | 3-5 天 | Protobuf 协议文件 |
| **阶段 1**: 网关核心框架 | 7-10 天 | 可运行的网关服务 |
| **阶段 2**: Go 聊天服务 (GCS) | 10-14 天 | 完整的聊天服务 |
| **阶段 3**: 网关与后端集成 | 5-7 天 | 端到端消息路由 |
| **阶段 4**: 客户端适配 | 7-10 天 | 客户端 SDK |
| **阶段 5**: 压力测试与优化 | 5-7 天 | 性能报告 |
| **阶段 6**: 生产就绪 | 3-5 天 | 部署方案与监控 |

---

## 阶段 0: 协议设计与定义 (3-5 天)

### 目标
定义网关层和业务层的 Protobuf 协议，确保所有服务和客户端共享统一的消息格式。

### 任务清单

#### 0.1 创建协议仓库结构
- [ ] 创建独立的协议仓库 `game-protocols`
  ```
  game-protocols/
  ├── gateway/
  │   └── envelope.proto          # 网关层协议
  ├── chat/
  │   ├── chat_message.proto      # 聊天消息
  │   ├── chat_service.proto      # 聊天服务 gRPC 定义
  │   └── chat_types.proto        # 聊天通用类型
  ├── game/
  │   ├── player.proto            # 玩家相关
  │   ├── combat.proto            # 战斗相关
  │   └── ...
  └── scripts/
      ├── generate_go.sh          # 生成 Go 代码
      ├── generate_cpp.sh         # 生成 C++ 代码
      └── generate_csharp.sh      # 生成 C# 代码（Unity）
  ```

#### 0.2 定义网关层协议 (`envelope.proto`)
- [ ] 定义 `Envelope` 消息结构
  ```protobuf
  syntax = "proto3";
  package gateway;
  
  option go_package = "github.com/yourorg/game-protocols/gateway";
  
  message Envelope {
      // 路由类型
      enum RouteType {
          UNKNOWN = 0;
          GAME = 1;        // 游戏逻辑
          CHAT = 2;        // 聊天
          SYSTEM = 3;      // 系统消息（心跳等）
      }
      
      RouteType route = 1;      // 路由目标
      uint64 sequence = 2;      // 消息序列号（用于响应匹配）
      bytes payload = 3;        // 业务消息的序列化字节
      
      // 可选：用于调试和追踪
      string trace_id = 4;      // 分布式追踪 ID
      int64 timestamp = 5;      // 客户端发送时间戳
  }
  ```

#### 0.3 定义聊天业务协议 (`chat/`)
- [ ] 定义聊天消息类型
  ```protobuf
  // chat/chat_message.proto
  syntax = "proto3";
  package chat;
  
  message ChatRequest {
      enum MessageType {
          TEXT = 0;           // 纯文本
          EMOJI = 1;          // 表情
          ITEM = 2;           // 道具
          COORDINATE = 3;     // 坐标
      }
      
      int32 sender_id = 1;
      int32 receiver_id = 2;        // 私聊接收者（0 表示频道消息）
      int32 channel_id = 3;         // 频道 ID（0 表示私聊）
      MessageType type = 4;
      string content = 5;
      bytes extra_data = 6;         // 附加数据（如道具信息）
  }
  
  message ChatResponse {
      bool success = 1;
      string error_message = 2;
      int64 message_id = 3;         // 消息在 DB 中的唯一 ID
      int64 timestamp = 4;
  }
  
  message MessageBroadcast {
      int64 message_id = 1;
      int32 sender_id = 2;
      string sender_name = 3;
      int32 channel_id = 4;
      string content = 5;
      int64 timestamp = 6;
  }
  ```

- [ ] 定义聊天服务 gRPC 接口
  ```protobuf
  // chat/chat_service.proto
  syntax = "proto3";
  package chat;
  
  service ChatService {
      // 网关 → GCS: 验证认证 Token
      rpc ValidateAuthToken(AuthTokenRequest) returns (UserIdentity);
      
      // GLS → GCS: 系统广播
      rpc SendSystemBroadcast(SystemBroadcastRequest) returns (Empty);
      
      // GLS → GCS: 踢出用户
      rpc KickUser(KickUserRequest) returns (Empty);
  }
  ```

#### 0.4 定义系统协议
- [ ] 心跳协议
  ```protobuf
  message Heartbeat {
      int64 client_timestamp = 1;
  }
  
  message HeartbeatResponse {
      int64 server_timestamp = 1;
  }
  ```

#### 0.5 生成代码
- [ ] 编写代码生成脚本
- [ ] 生成 Go 代码（网关 + GCS）
- [ ] 生成 C++ 代码（GLS）
- [ ] 生成 C# 代码（Unity 客户端）

#### 0.6 多游戏通用化设计 ⭐

**核心思想**: 通过定义 **MessageBase** 基础结构，将通用字段（如 game_id、user_id）提取到基类，所有业务消息继承该基类，实现跨游戏复用。

> 📖 **详细设计请参考**: [`docs/multi_game_architecture.md`](./multi_game_architecture.md)

##### 关键要点

1. **MessageBase 基类**
   ```protobuf
   message MessageBase {
       string game_id = 1;       // 游戏标识
       int32 user_id = 2;        // 用户 ID
       int64 timestamp = 3;      // 时间戳
       string trace_id = 6;      // 追踪 ID
   }
   ```

2. **业务消息组合 Base**
   ```protobuf
   message ChatRequest {
       MessageBase base = 1;     // 包含基类
       string content = 2;       // 业务字段
   }
   ```

3. **Envelope 添加 game_id**
   ```protobuf
   message Envelope {
       RouteType route = 1;
       bytes payload = 3;
       string game_id = 6;       // 避免解析 payload
   }
   ```

##### 任务清单

- [ ] 定义 `common/message_base.proto`
- [ ] 在 `Envelope` 中添加 `game_id` 字段
- [ ] 所有业务消息包含 `MessageBase`
- [ ] 网关实现基于 `game_id` 的路由逻辑
- [ ] GCS 数据库表添加 `game_id` 字段和索引
- [ ] 编写多游戏配置示例
- [ ] 文档：新游戏接入指南（参考 multi_game_architecture.md）

### 交付物
- ✅ 完整的 `.proto` 文件
- ✅ 多语言生成的代码（Go/C++/C#）
- ✅ 协议文档（每个消息的用途说明）

### 验收标准
- [ ] 所有 `.proto` 文件编译无错误
- [ ] 生成的代码能在各自环境中正常导入
- [ ] 团队评审通过

---

## 阶段 1: 网关核心框架 (7-10 天)

### 目标
构建一个可运行的网关服务，能够接受客户端 WebSocket 连接、解析 Envelope、并维护到后端的连接池。

### 任务清单

#### 1.1 项目初始化
- [ ] 创建网关项目 `game-gateway`
  ```
  game-gateway/
  ├── cmd/
  │   └── gateway/
  │       └── main.go
  ├── internal/
  │   ├── config/           # 配置管理
  │   ├── server/           # WebSocket 服务器
  │   ├── router/           # 消息路由逻辑
  │   ├── backend/          # 后端连接池
  │   └── session/          # 客户端会话管理
  ├── pkg/
  │   └── middleware/       # 中间件（认证、限流等）
  ├── configs/
  │   └── gateway.yaml
  └── go.mod
  ```

- [ ] 依赖管理
  ```go
  // 核心依赖
  github.com/gorilla/websocket
  google.golang.org/protobuf
  google.golang.org/grpc
  github.com/go-redis/redis/v8
  github.com/sirupsen/logrus
  ```

#### 1.2 配置系统
- [ ] 定义配置结构
  ```go
  type Config struct {
      Server struct {
          Host string `yaml:"host"`
          Port int    `yaml:"port"`
      }
      Backend struct {
          GameServer struct {
              Host string `yaml:"host"`
              Port int    `yaml:"port"`
          }
          ChatServer struct {
              Host string `yaml:"host"`
              Port int    `yaml:"port"`
          }
      }
      Redis struct {
          Addr     string `yaml:"addr"`
          Password string `yaml:"password"`
      }
  }
  ```

#### 1.3 WebSocket 服务器
- [ ] 实现 WebSocket 连接处理
  ```go
  type Server struct {
      upgrader websocket.Upgrader
      router   *Router
      sessions *SessionManager
  }
  
  func (s *Server) HandleConnection(w http.ResponseWriter, r *http.Request) {
      conn, _ := s.upgrader.Upgrade(w, r, nil)
      session := s.sessions.CreateSession(conn)
      
      go s.readLoop(session)
      go s.writeLoop(session)
  }
  ```

#### 1.4 会话管理
- [ ] 实现客户端会话结构
  ```go
  type Session struct {
      ID           string
      ClientConn   *websocket.Conn
      GameConn     *BackendConnection
      ChatConn     *BackendConnection
      SendQueue    chan []byte
      AuthToken    string
      UserID       int32
      CreatedAt    time.Time
  }
  ```

- [ ] 会话生命周期管理
  - [ ] 会话创建
  - [ ] 会话清理（超时、断线）
  - [ ] 并发安全的会话存储（sync.Map 或 Redis）

#### 1.5 后端连接池
- [ ] 实现后端连接池
  ```go
  type BackendPool struct {
      address     string
      connections chan *BackendConnection
      maxConn     int
  }
  
  type BackendConnection struct {
      conn      *websocket.Conn  // 或 TCP 连接
      available bool
      lastUsed  time.Time
  }
  ```

- [ ] 连接池功能
  - [ ] 连接获取与归还
  - [ ] 连接健康检查
  - [ ] 自动重连机制

#### 1.6 消息路由器
- [ ] 实现 Envelope 解析与路由
  ```go
  type Router struct {
      gameBackend *BackendPool
      chatBackend *BackendPool
  }
  
  func (r *Router) RouteMessage(session *Session, data []byte) error {
      // 1. 解析 Envelope
      var envelope gateway.Envelope
      proto.Unmarshal(data, &envelope)
      
      // 2. 根据 route 字段转发
      switch envelope.Route {
      case gateway.Envelope_GAME:
          return r.forwardToGame(session, envelope.Payload)
      case gateway.Envelope_CHAT:
          return r.forwardToChat(session, envelope.Payload)
      case gateway.Envelope_SYSTEM:
          return r.handleSystem(session, envelope.Payload)
      }
  }
  ```

#### 1.7 基础中间件
- [ ] 连接限流（Rate Limiting）
- [ ] 请求日志（每条消息的路由记录）
- [ ] 错误处理与优雅降级

### 交付物
- ✅ 可运行的网关服务
- ✅ 支持 WebSocket 连接
- ✅ 能解析 Envelope 并打印路由信息

### 验收标准
- [ ] 网关能启动并监听端口
- [ ] WebSocket 客户端能成功连接
- [ ] 发送 Envelope 消息后能正确解析 route 字段
- [ ] 日志记录清晰（连接、断开、消息路由）

---

## 阶段 2: Go 聊天服务 (GCS) (10-14 天)

### 目标
实现完整的聊天服务，包括实时消息路由、离线消息、历史记录、数据持久化。

### 任务清单

#### 2.1 项目初始化
- [ ] 创建聊天服务项目 `game-chat-service`
  ```
  game-chat-service/
  ├── cmd/
  │   └── chat/
  │       └── main.go
  ├── internal/
  │   ├── handler/          # 消息处理器
  │   ├── hub/              # 连接管理中心
  │   ├── service/          # 业务逻辑
  │   ├── repository/       # 数据访问层
  │   └── grpc/             # gRPC 服务实现
  ├── pkg/
  │   └── middleware/
  ├── migrations/           # 数据库迁移脚本
  └── configs/
  ```

#### 2.2 数据库设计
- [ ] 设计消息表
  ```sql
  CREATE TABLE messages (
      id BIGSERIAL PRIMARY KEY,
      sender_id INT NOT NULL,
      receiver_id INT,              -- NULL for channel messages
      channel_id INT,               -- NULL for private messages
      content TEXT NOT NULL,
      message_type INT DEFAULT 0,   -- TEXT, EMOJI, ITEM, etc.
      extra_data BYTEA,
      is_read BOOLEAN DEFAULT FALSE,
      created_at TIMESTAMP DEFAULT NOW(),
      INDEX idx_receiver_unread (receiver_id, is_read),
      INDEX idx_channel_time (channel_id, created_at)
  );
  ```

- [ ] 设计用户在线状态表（Redis）
  ```
  Key: user:online:{user_id}
  Value: {
      "session_id": "xxx",
      "gcs_instance": "gcs-1",
      "connected_at": 1234567890
  }
  TTL: 300 秒（心跳更新）
  ```

#### 2.3 Hub 连接管理
- [ ] 实现 Hub 结构
  ```go
  type Hub struct {
      clients    map[int32]*Client  // userID -> Client
      channels   map[int32]*Channel // channelID -> Channel
      register   chan *Client
      unregister chan *Client
      broadcast  chan *BroadcastMsg
  }
  
  type Client struct {
      UserID    int32
      Conn      net.Conn  // 来自网关的连接
      Send      chan []byte
      Hub       *Hub
  }
  ```

- [ ] 实现 Hub 运行逻辑
  ```go
  func (h *Hub) Run() {
      for {
          select {
          case client := <-h.register:
              h.clients[client.UserID] = client
              h.updateRedisPresence(client.UserID, true)
          case client := <-h.unregister:
              delete(h.clients, client.UserID)
              h.updateRedisPresence(client.UserID, false)
          case msg := <-h.broadcast:
              h.broadcastToChannel(msg)
          }
      }
  }
  ```

#### 2.4 消息处理器
- [ ] 私聊消息处理
  ```go
  func (s *Service) HandlePrivateMessage(req *chat.ChatRequest) error {
      // 1. 持久化到数据库
      msgID := s.repo.SaveMessage(req)
      
      // 2. 检查接收者是否在线
      if s.isUserOnline(req.ReceiverId) {
          // 实时推送
          s.hub.SendToUser(req.ReceiverId, msgData)
      } else {
          // 标记为未读
          s.repo.MarkUnread(msgID)
      }
      
      return nil
  }
  ```

- [ ] 频道消息处理
- [ ] 离线消息拉取
  ```go
  func (s *Service) PullOfflineMessages(userID int32) ([]*chat.MessageBroadcast, error) {
      return s.repo.GetUnreadMessages(userID)
  }
  ```

#### 2.5 gRPC 服务实现
- [ ] 实现 `ValidateAuthToken`（供网关调用）
  ```go
  func (s *GrpcService) ValidateAuthToken(ctx context.Context, req *AuthTokenRequest) (*UserIdentity, error) {
      // 调用 C++ GLS 的认证接口
      identity, err := s.glsClient.VerifyToken(req.Token)
      return identity, err
  }
  ```

- [ ] 实现 `SendSystemBroadcast`（供 GLS 调用）
- [ ] 实现 `KickUser`（供 GLS 调用）

#### 2.6 Redis Pub/Sub（跨实例通信）
- [ ] 实现消息发布
  ```go
  func (s *Service) PublishToRedis(channel string, msg []byte) {
      s.redisClient.Publish(ctx, channel, msg)
  }
  ```

- [ ] 实现消息订阅
  ```go
  func (s *Service) SubscribeRedis() {
      pubsub := s.redisClient.Subscribe(ctx, "chat:broadcast")
      for msg := range pubsub.Channel() {
          s.handleCrossInstanceMessage(msg.Payload)
      }
  }
  ```

#### 2.7 数据持久化层
- [ ] 实现 Repository 接口
  ```go
  type Repository interface {
      SaveMessage(msg *chat.ChatRequest) (int64, error)
      GetUnreadMessages(userID int32) ([]*Message, error)
      MarkAsRead(userID int32, messageIDs []int64) error
      GetChannelHistory(channelID int32, limit int, beforeID int64) ([]*Message, error)
  }
  ```

- [ ] PostgreSQL 实现
- [ ] 添加数据库连接池
- [ ] 添加查询超时控制

### 交付物
- ✅ 完整的聊天服务
- ✅ 支持私聊和频道消息
- ✅ 支持离线消息
- ✅ gRPC 接口可供网关和 GLS 调用

### 验收标准
- [ ] 单元测试覆盖率 > 70%
- [ ] 能处理基本的聊天场景（私聊、频道、离线）
- [ ] gRPC 接口测试通过
- [ ] 数据库能正确存储和查询消息

---

## 阶段 3: 网关与后端集成 (5-7 天)

### 目标
将网关与 GCS 和 GLS 完全打通，实现端到端的消息流转。

### 任务清单

#### 3.1 网关 ↔ GCS 集成
- [ ] 网关建立到 GCS 的连接池
  - [ ] WebSocket 连接（实时消息）
  - [ ] gRPC 连接（认证调用）

- [ ] 实现双向消息转发
  ```go
  // 客户端 → 网关 → GCS
  func (r *Router) forwardToChat(session *Session, payload []byte) error {
      return session.ChatConn.WriteMessage(websocket.BinaryMessage, payload)
  }
  
  // GCS → 网关 → 客户端
  func (r *Router) listenChatResponses(session *Session) {
      for {
          _, data, _ := session.ChatConn.ReadMessage()
          // 重新封装成 Envelope
          envelope := &gateway.Envelope{
              Route: gateway.Envelope_CHAT,
              Payload: data,
          }
          session.SendQueue <- marshalEnvelope(envelope)
      }
  }
  ```

#### 3.2 认证流程集成
- [ ] 客户端连接网关时，网关调用 GCS 的 `ValidateAuthToken`
  ```go
  func (s *Server) authenticateSession(token string) (*UserIdentity, error) {
      conn, _ := grpc.Dial(s.config.ChatServer.GrpcAddr)
      client := chat.NewChatServiceClient(conn)
      
      resp, err := client.ValidateAuthToken(ctx, &AuthTokenRequest{
          Token: token,
      })
      return resp, err
  }
  ```

- [ ] 认证成功后，网关建立到 GCS 的长连接

#### 3.3 网关 ↔ GLS 集成
- [ ] 游戏逻辑消息转发
- [ ] 系统消息路由

#### 3.4 错误处理与重试
- [ ] 后端服务不可用时的降级策略
- [ ] 消息重试机制（指数退避）
- [ ] 熔断器（Circuit Breaker）

#### 3.5 端到端测试
- [ ] 模拟客户端发送聊天消息
- [ ] 验证消息完整路径：客户端 → 网关 → GCS → DB → GCS → 网关 → 客户端
- [ ] 测试离线消息场景

### 交付物
- ✅ 网关与 GCS 完全打通
- ✅ 能处理完整的聊天流程

### 验收标准
- [ ] 端到端测试全部通过
- [ ] 消息延迟 < 100ms (本地测试)
- [ ] 无消息丢失

---

## 阶段 4: 客户端适配 (7-10 天)

### 目标
提供客户端 SDK，封装 Envelope 的序列化和反序列化逻辑。

### 任务清单

#### 4.1 Unity C# SDK
- [ ] 创建 SDK 项目
  ```
  GameSDK/
  ├── Gateway/
  │   ├── GatewayClient.cs
  │   └── EnvelopeHelper.cs
  ├── Chat/
  │   ├── ChatClient.cs
  │   └── ChatMessage.cs
  └── Proto/                  # 生成的 Protobuf 代码
  ```

- [ ] 实现 GatewayClient
  ```csharp
  public class GatewayClient {
      private WebSocket ws;
      private Dictionary<RouteType, Action<byte[]>> handlers;
      
      public void Connect(string url, string authToken) {
          ws = new WebSocket(url);
          ws.OnMessage += OnMessage;
          ws.Connect();
          // 发送认证消息
      }
      
      public void SendChatMessage(string content) {
          var chatReq = new ChatRequest { Content = content };
          var envelope = new Envelope {
              Route = RouteType.Chat,
              Payload = chatReq.ToByteArray()
          };
          ws.Send(envelope.ToByteArray());
      }
      
      private void OnMessage(byte[] data) {
          var envelope = Envelope.Parser.ParseFrom(data);
          handlers[envelope.Route]?.Invoke(envelope.Payload);
      }
  }
  ```

- [ ] 实现 ChatClient（高层封装）
  ```csharp
  public class ChatClient {
      private GatewayClient gateway;
      
      public event Action<ChatMessage> OnMessageReceived;
      
      public void SendMessage(string content, int receiverId) {
          var req = new ChatRequest {
              Content = content,
              ReceiverId = receiverId
          };
          gateway.SendChatMessage(req);
      }
  }
  ```

#### 4.2 其他客户端平台（可选）
- [ ] Web 客户端（JavaScript）
- [ ] C++ 客户端（Unreal Engine）

#### 4.3 示例项目
- [ ] 创建 Unity Demo 场景
  - [ ] 简单的聊天界面
  - [ ] 发送/接收消息
  - [ ] 显示在线状态

### 交付物
- ✅ Unity C# SDK
- ✅ SDK 使用文档
- ✅ Demo 项目

### 验收标准
- [ ] 客户端能成功连接网关
- [ ] 能发送和接收聊天消息
- [ ] 代码易用性良好（接口简洁）

---

## 阶段 5: 压力测试与优化 (5-7 天)

### 目标
验证系统在高并发下的性能，找出瓶颈并优化。

### 任务清单

#### 5.1 测试环境准备
- [ ] 搭建测试集群
  - 网关 x 2
  - GCS x 2
  - PostgreSQL
  - Redis

#### 5.2 压力测试
- [ ] 编写压测脚本（使用 Go 或 JMeter）
  ```go
  // 模拟 10000 个并发用户
  for i := 0; i < 10000; i++ {
      go func(userID int) {
          client := NewTestClient()
          client.Connect()
          // 定期发送消息
          ticker := time.NewTicker(5 * time.Second)
          for range ticker.C {
              client.SendMessage("Hello")
          }
      }(i)
  }
  ```

- [ ] 测试场景
  - [ ] 10K 并发连接
  - [ ] 1K QPS 消息吞吐
  - [ ] 频道广播（1 对 1000）
  - [ ] 离线消息积压

#### 5.3 性能指标收集
- [ ] 延迟（P50, P95, P99）
- [ ] CPU/内存使用率
- [ ] 数据库 QPS
- [ ] Redis 命中率

#### 5.4 优化
- [ ] 网关优化
  - [ ] Goroutine 池优化
  - [ ] 减少内存分配（sync.Pool）
  - [ ] 消息批量处理

- [ ] GCS 优化
  - [ ] 数据库连接池调优
  - [ ] Redis Pipeline
  - [ ] 消息批量写入

- [ ] 数据库优化
  - [ ] 添加索引
  - [ ] 分表策略（按时间或用户 ID）

### 交付物
- ✅ 压测报告
- ✅ 性能优化方案

### 验收标准
- [ ] 支持 10K+ 并发连接
- [ ] 消息延迟 P99 < 500ms
- [ ] 系统稳定运行 24 小时无崩溃

---

## 阶段 6: 生产就绪 (3-5 天)

### 目标
完善监控、日志、部署方案，确保系统可以安全上线。

### 任务清单

#### 6.1 监控与告警
- [ ] 接入 Prometheus + Grafana
  - [ ] 网关指标（连接数、QPS、延迟）
  - [ ] GCS 指标（在线用户、消息吞吐、DB 延迟）
  
- [ ] 设置告警规则
  - [ ] CPU/内存超过 80%
  - [ ] 消息延迟 P99 > 1s
  - [ ] 数据库连接池耗尽

#### 6.2 日志系统
- [ ] 结构化日志（JSON 格式）
- [ ] 日志聚合（ELK 或 Loki）
- [ ] 分布式追踪（Jaeger）

#### 6.3 部署方案
- [ ] Docker 镜像构建
  ```dockerfile
  FROM golang:1.21 AS builder
  WORKDIR /app
  COPY . .
  RUN go build -o gateway ./cmd/gateway
  
  FROM alpine:latest
  COPY --from=builder /app/gateway /gateway
  CMD ["/gateway"]
  ```

- [ ] Kubernetes 部署清单
  ```yaml
  apiVersion: apps/v1
  kind: Deployment
  metadata:
    name: gateway
  spec:
    replicas: 3
    template:
      spec:
        containers:
        - name: gateway
          image: your-registry/gateway:latest
          ports:
          - containerPort: 8080
  ```

- [ ] 灰度发布方案

#### 6.4 安全加固
- [ ] TLS/SSL（WebSocket Secure）
- [ ] Token 加密存储
- [ ] 限流与防 DDoS

#### 6.5 文档完善
- [ ] 架构设计文档
- [ ] API 文档
- [ ] 运维手册
- [ ] 故障处理 Runbook

### 交付物
- ✅ 完整的部署方案
- ✅ 监控面板
- ✅ 运维文档

### 验收标准
- [ ] 能一键部署到生产环境
- [ ] 监控覆盖所有关键指标
- [ ] 故障恢复时间 < 5 分钟

---

## 风险与应对

| 风险 | 影响 | 应对措施 |
|------|------|---------|
| 网关成为单点故障 | 高 | 部署多个网关实例 + 负载均衡 |
| 消息丢失 | 高 | 消息持久化 + 客户端重试 + 消息 ACK 机制 |
| 性能不达预期 | 中 | 提前压测，预留性能优化时间 |
| 协议变更导致不兼容 | 中 | Protobuf 版本管理 + 向后兼容策略 |
| 数据库成为瓶颈 | 中 | 读写分离 + 分表 + 缓存 |

---

## 里程碑检查点

- **Week 1 结束**: 协议定义完成，代码生成成功
- **Week 2 结束**: 网关能接受连接并解析 Envelope
- **Week 3 结束**: GCS 基本功能实现（私聊、频道）
- **Week 4 结束**: 网关与 GCS 集成，端到端打通
- **Week 5 结束**: 客户端 SDK 完成，Demo 可运行
- **Week 6 结束**: 压测通过，生产就绪

---

## 附录

### A. 快速启动命令

```bash
# 启动网关
cd game-gateway
go run cmd/gateway/main.go --config configs/gateway.yaml

# 启动聊天服务
cd game-chat-service
go run cmd/chat/main.go --config configs/chat.yaml

# 运行测试
go test ./... -v

# 构建 Docker 镜像
docker build -t game-gateway:latest .
```

### B. 常用调试技巧

1. **查看 Envelope 原始数据**
   ```bash
   # 使用 Wireshark 抓包
   # 使用 protoc 解码
   protoc --decode=gateway.Envelope envelope.proto < message.bin
   ```

2. **查看 Redis 订阅**
   ```bash
   redis-cli PSUBSCRIBE 'chat:*'
   ```

3. **数据库慢查询**
   ```sql
   SELECT * FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;
   ```

---

**最后更新**: 2025-12-17
**负责人**: [填写]
**状态**: 待启动
