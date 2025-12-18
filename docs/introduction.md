# 通用游戏聊天系统

```plantuml {kroki=true}
@startuml 游戏系统部署架构
!theme vibrant
skinparam linetype ortho


left to right direction

' 定义样式
skinparam node {
    BackgroundColor White
    BorderColor Black
    FontSize 12
}

skinparam artifact {
    BackgroundColor<<client>> LightBlue
    BackgroundColor<<gateway>> LightGreen
    BackgroundColor<<mq>> Gold
    BackgroundColor<<service>> LightCoral
    BackgroundColor<<database>> Yellow
}

' 客户端节点
artifact "游戏客户端\n(10000+)" as client_app <<client>>

' 网关层
artifact "网关服\n(Gateway)" as gateway <<gateway>>

' 消息队列层
artifact "Redis 消息队列\n(Pub/Sub)" as mq <<mq>>

' 服务层
artifact "聊天服\n(Chat Service)\n并发消费" as chat <<service>>

' 数据层
artifact "数据库\n(PostgreSQL)\n异步写入" as postgres <<database>>
artifact "缓存\n(Redis)" as redis <<database>>

' 强制水平排列
client_app -[hidden]right-> gateway
gateway -[hidden]right-> mq
mq -[hidden]right-> chat
chat -[hidden]right-> postgres
postgres -[hidden]right-> redis

' 请求流向 (客户端 -> Gateway -> MQ -> Chat Service)
client_app --> gateway
gateway --> mq
mq --> chat

' 响应流向 (Chat Service -> MQ -> Gateway -> 客户端)
chat --> mq
mq --> gateway
gateway --> client_app

' 持久化
chat --> postgres
chat --> redis

@enduml
```

## 跨游戏通用平台

### 核心设计
- **Gateway**: 只解析协议头（8字节），根据 `game_id` 路由到不同 Topic
- **隔离机制**: 消息队列 Topic 隔离 + 数据库 `game_id` 字段 + Redis Key 前缀
- **字段扩展**: Protobuf `oneof` 支持游戏专属字段

### 性能
10000+ 并发 | 2.2ms 延迟 | 100% 成功率

### 新游戏接入（3步）

**Step 1**: 配置文件添加游戏
```yaml
games:
  - id: "moba"
    chat_backend: {host: "localhost", port: 9002}
```

**Step 2**: （可选）扩展专属字段
```protobuf
message ChatRequest {
    MessageBase base = 1;
    oneof game_specific {
        MOBAExtra moba_extra = 12;  // 新增
    }
}
```

**Step 3**: 编译重启
```bash
make proto && make restart-app
```


