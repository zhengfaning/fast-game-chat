# 游戏聊天系统架构 (v2.0)

## 概述
游戏聊天系统已演进为完全异步、事件驱动的架构，专为高并发设计（经测试可支持 10,000+ 同时在线用户，且延迟 <5ms）。

核心设计哲学是**解耦**：
1. **客户端连接与业务逻辑解耦**（通过网关 Gateway）。
2. **网关与服务解耦**（通过 Redis 消息队列）。
3. **请求处理与数据持久化解耦**（通过 Write-Behind 异步写入）。

## 核心组件

### 1. 游戏网关 Gateway (无状态连接器)
- **角色**：管理与客户端的 WebSocket 连接。
- **行为**：
    - **入站 (Inbound)**：接收 WebSocket 数据包，封装后**发布 (Publish)** 到 Redis 主题 `game:request:{gameID}`。
    - **出站 (Outbound)**：**订阅 (Subscribe)** Redis 主题 `broadcast`。根据本地会话表 (Session Map) 将接收到的消息路由给特定用户。
- **关键特性**：与聊天服务 (Chat Service) 无直接 TCP 连接，无线程阻塞。

### 2. Redis 消息总线 (骨干网络)
- **角色**：高吞吐量消息代理。
- **主题 (Topics)**：
    - `game:request:{gameID}`：上行请求（客户端 -> 服务器）。
    - `broadcast`：下行响应和通知（服务器 -> 客户端）。

### 3. 聊天服务 Chat Service (并发处理器)
- **角色**：处理业务逻辑（过滤、存储、路由计算）。
- **并发模型**：
    - **每个请求一个 Goroutine**：为每个进入的 Redis 消息启动轻量级 Goroutine。
    - **非阻塞**：不等待数据库写入完成。
- **持久化策略 (Write-Behind)**：
    - 使用大容量缓冲通道 (`saveChan`) 立即接收保存请求。
    - 背景工作池 (Worker Pool) 消费该通道并对 PostgreSQL 执行 `INSERT`。
    - **优势**：即使数据库负载较高，用户也能体验到毫秒级的延迟。

## 数据流

### 请求路径 (上行)
1. **客户端** 发送 `ChatRequest`。
2. **网关 (Gateway)** 接收 -> 发布到 Redis `game:request:mmo`。
3. **聊天服务 (Chat Service)** 消费 -> 启动 Goroutine -> 推送至 DB 通道 -> 发布 `ChatResponse` 到 Redis。

### 响应路径 (下行)
1. **聊天服务 (Chat Service)** 发布 `ChatResponse` (确认) 或 `MessageBroadcast` 到 Redis `broadcast`。
2. **网关 (Gateway)** 接收 -> 查找会话 -> 写入 WebSocket。
3. **客户端** 接收消息。

## 性能特性
- **吞吐量**：已验证 10,000 名并发用户在 <30 秒内生成 20,000 个请求。
- **延迟**：平均往返延迟 **~2.2ms**。
- **扩展性**：
    - **网关 (Gateway)**：水平可扩展（只需增加更多节点，它们都会订阅 `broadcast`）。
    - **聊天服务 (Chat Service)**：水平可扩展（增加节点以消费 Redis 队列 - *注：简单的发布/订阅模式会重复消息，对于多个服务实例，建议未来升级使用 Redis Streams 或分片主题*）。
