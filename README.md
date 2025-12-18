# 🎮 Game Dev Platform - Multi-Game Universal Architecture

![Build Status](https://img.shields.io/badge/build-passing-brightgreen)
![Go Version](https://img.shields.io/badge/go-1.24-blue)
![Architecture](https://img.shields.io/badge/architecture-multi--game-orange)

这是一个为多游戏环境打造的高性能、可扩展的通用后端基础设施。通过统一的网关和聊天服务，支持多个游戏项目复用同一套基础设施，实现代码复用、数据隔离和极速接入。这是针对乐狗科技的要求：“设计、开发和维护跨多个游戏的核心平台功能，如聊天系统和邮件系统”而编写的通用性聊天服。（本人正在找工作，如果需要可以联系我：zhengfaning@hotmail.com）

## ✨ 核心特性

- 🌐 **快速网关 (Gateway)**: 只需要解析消息头二进制协议，支持多游戏动态路由，具备极高的并发处理能力。
- 💬 **智能聊天服务 (Chat Service)**: 原生支持多游戏数据隔离，内置私聊、频道广播功能。
- 🛡️ **数据隔离**: 基于 `game_id` 的存储与逻辑隔离，确保各游戏数据互不干扰。
- 📦 **开箱即用**: 完整的 Docker 化环境配置，通过 `Makefile` 实现一键流水线作业。
- 🧪 **压力测试验证**: 已通过单机 **10,000+** 并发用户的严苛测试，**100% 成功率**，平均响应延迟仅为 **2.2ms**。
- ⚡ **高性能架构**: 采用消息队列解耦逻辑、并发消息消费以及异步持久化（Write-Behind）技术，完美消除 I/O 瓶颈。
- 🔌 **消息队列抽象**: 核心通信层已针对消息队列进行高度抽象（`mq.Producer/Consumer` 接口）。当前基于 **Redis Pub/Sub** 实现，但可根据业务规模无缝切换至 **Kafka**、**RabbitMQ** 等专业中间件。

## 🚀 性能表现 (Performance)

系统在单机环境下（16核 CPU, 16GB内存）通过了极限压力测试，核心指标如下：

| 指标 | 测试数据 | 说明 |
| :--- | :--- | :--- |
| **并发连接数** | **10,000** | 稳定维持万级 WebSocket 长连接 |
| **消息成功率** | **100.00%** | 在极端建立连接风暴下无一丢包 |
| **平均延迟** | **2.20 ms** | 从网关接收到业务处理完成的全链路耗时 |
| **最大延迟** | **88.89 ms** | 即使在峰值波动下也能保持极快响应 |
| **消息吞吐量** | **830+ req/s** | 包含极速连接建立过程中的逻辑交互（相互发聊天消息） |

详细测试报告请参阅：[10000用户高并发测试报告](docs/STRESS_TEST_10000_USERS_REPORT.md)

## 🏗️ 系统架构

项目的核心理念是 **MessageBase + game_id**。所有业务消息继承通用基类，网关根据消息体中的标识进行智能路由。

```text
客户端 A (MMO) ──┐
                 │    连接 (WS)    ┌──────────┐    发布 (Pub)    ┌──────────┐    消费 (Sub)    ┌──────────┐
客户端 B (Card) ─┼───────────────→ │ 统一网关 │  ────────────→  │ Redis MQ │  ────────────→  │ 聊天服务 │
                 │                 │ Gateway  │  ←────────────  │  (消息总线) │  ←────────────  │ Chat Svc │
客户端 C (MOBA) ──┘                 └──────────┘    广播 (Sub)    └──────────┘    发布 (Pub)    └──────────┘
                                                                                                    │
                                                                                               异步持久化 ↓
                                                                                               [ PostgreSQL ]
```

## 🧩 通用性与极速接入 (Extensibility)

本项目的设计核心是**“协议驱动且游戏无关”**。当需要接入一款新游戏（如：Card Game）时，无需修改网关或聊天服务的核心代码：

1. **配置驱动 (Configuration)**:
   - 仅需在 `gateway.yaml` 中增加一条游戏配置，指定该游戏的 `game_id` 以及后端服务地址。
2. **协议复用 (Protocol)**:
   - 复用 `game-protocols` 中的通用消息头（包含 `game_id` 和 `user_id`）。
   - 新游戏的业务逻辑只需关注其自身的 Protobuf 业务 Payload。
3. **逻辑隔离 (Isolation)**:
   - 聊天服务会自动根据 `game_id` 分配 Redis 频道并隔离数据库存储。
4. **水平扩展 (Scaling)**:
   - 可以为特定高压力游戏部署独占的聊天服务实例，通过订阅特定的 `game:request:{game_id}` 队列实现负载隔离。

## 📂 目录结构

```text
.
├── docker/                 # Docker 基础设施配置 (PG, Redis, etc.)
├── dist/                   # 构建发布目录 (自动化生成)
├── game-gateway/           # 高性能网关服务源码
├── game-chat-service/      # 通用聊天服务源码
├── game-protocols/         # 基于 Protobuf 的跨语言协议定义
├── scripts/                # 压力测试与辅助工具脚本
├── docs/                   # 详细的设计文档与技术手册
└── Makefile                # 统一的项目管理命令
```

## 🚀 快速开始

### 1. 环境准备
确保您的机器已安装 `docker`, `docker-compose` 和 `go`。

### 2. 启动基础设施
```bash
make docker-up
```

### 3. 编译并运行应用
```bash
make release  # 编译并打包至 dist
make run      # 后台启动所有服务
```

### 4. 状态检查
```bash
make docker-ps  # 查看容器状态
make test-db    # 测试数据库连接
make test-redis # 测试 Redis 连接
```

### 5. 压力测试（可选）
```bash
make test-stress          # 默认 1000 用户
make test-stress USERS=500   # 自定义 500 用户
make test-stress USERS=2000  # 自定义 2000 用户
```

## 🛠️ 管理命令手册

| 命令 | 说明 |
| :--- | :--- |
| `make build` | 编译所有微服务 |
| `make release` | 编译并打包到 dist 目录（配置文件仅在不存在时复制） |
| `make run` | 后台启动所有应用服务 |
| `make stop` | 停止所有应用进程 |
| `make restart-app` | 重启所有应用 |
| `make docker-up` | 启动 Docker 基础服务 (PostgreSQL, Redis) |
| `make docker-down` | 停止 Docker 服务 |
| `make docker-logs` | 查看容器运行日志 |
| `make psql` | 进入 PostgreSQL 终端 |
| `make redis-cli` | 进入 Redis 终端 |
| `make test-db` | 测试数据库连接 |
| `make test-redis` | 测试 Redis 连接 |
| `make test-stress` | 运行压力测试（默认 1000 用户，可用 USERS=N 自定义） |
| `make clean` | 清理编译产物与发布包 |

## 📖 技术文档

深入了解系统细节：
- [多游戏通用架构设计说明](docs/multi_game_architecture.md)
- [高性能协议优化指南](docs/PROTOCOL_OPTIMIZATION.md)
- [系统组件关系图 (Architecture)](docs/architecture_diagram.md)

---

**文档版本**: v1.0.0  
**维护者**: Zheng Faning
